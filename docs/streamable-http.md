# Streamable HTTP Implementation Guide

## Overview

This document details the streaming implementation for the MCP Immich server using the Go MCP SDK with support for Server-Sent Events (SSE), chunked transfer encoding, and WebSocket protocols. The implementation automatically switches to streaming mode for large datasets (>500 items by default).

## Core Streaming Requirements

### 1. Protocol Support
- **HTTP/1.1**: Chunked Transfer Encoding
- **HTTP/2**: Stream multiplexing and server push
- **WebSocket**: Bidirectional streaming (optional)
- **Server-Sent Events (SSE)**: Unidirectional server-to-client streaming

### 2. Performance Requirements
- Maximum latency for first byte: 100ms
- Stream buffer size: 64KB - 256KB
- Concurrent streams: 100 per connection
- Backpressure threshold: 80% buffer capacity
- Keep-alive timeout: 120 seconds

## Transport Implementations

### 1. HTTP Chunked Transfer Encoding

Used for progressive delivery of large JSON responses:

```go
// pkg/transport/http_chunked.go
package transport

import (
    "encoding/json"
    "net/http"
    "github.com/modelcontextprotocol/go-sdk/transport"
)

type HTTPChunkedTransport struct {
    transport.Transport
    w       http.ResponseWriter
    r       *http.Request
    encoder *json.Encoder
    decoder *json.Decoder
    flusher http.Flusher
}

func NewHTTPChunkedTransport(w http.ResponseWriter, r *http.Request) *HTTPChunkedTransport {
    w.Header().Set("Transfer-Encoding", "chunked")
    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Cache-Control", "no-cache")

    flusher, _ := w.(http.Flusher)

    return &HTTPChunkedTransport{
        w:       w,
        r:       r,
        encoder: json.NewEncoder(w),
        decoder: json.NewDecoder(r.Body),
        flusher: flusher,
    }
}

func (t *HTTPChunkedTransport) Send(msg interface{}) error {
    if err := t.encoder.Encode(msg); err != nil {
        return err
    }

    if t.flusher != nil {
        t.flusher.Flush()
    }

    return nil
}

func (t *HTTPChunkedTransport) StreamBatch(items []interface{}) error {
    wrapper := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "stream/data",
        "params": map[string]interface{}{
            "items": items,
            "count": len(items),
        },
    }

    return t.Send(wrapper)
}
```

### 2. Server-Sent Events (SSE)

Primary streaming method for unidirectional server-to-client communication:

```go
// pkg/transport/http_sse.go
package transport

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/modelcontextprotocol/go-sdk/transport"
)

type HTTPSSETransport struct {
    transport.Transport
    w           http.ResponseWriter
    r           *http.Request
    flusher     http.Flusher
    scanner     *bufio.Scanner
    heartbeatTicker *time.Ticker
}

func NewHTTPSSETransport(w http.ResponseWriter, r *http.Request) (*HTTPSSETransport, error) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        return nil, fmt.Errorf("streaming not supported")
    }

    // Set SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no") // Disable Nginx buffering

    return &HTTPSSETransport{
        w:               w,
        r:               r,
        flusher:         flusher,
        scanner:         bufio.NewScanner(r.Body),
        heartbeatTicker: time.NewTicker(30 * time.Second),
    }, nil
}

func (t *HTTPSSETransport) Send(msg interface{}) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    fmt.Fprintf(t.w, "event: message\n")
    fmt.Fprintf(t.w, "data: %s\n\n", data)

    t.flusher.Flush()
    return nil
}

func (t *HTTPSSETransport) SendStream(event string, data interface{}) error {
    jsonData, err := json.Marshal(data)
    if err != nil {
        return err
    }

    fmt.Fprintf(t.w, "event: %s\n", event)
    fmt.Fprintf(t.w, "data: %s\n\n", jsonData)

    t.flusher.Flush()
    return nil
}

func (t *HTTPSSETransport) SendProgress(progress int, message string) error {
    return t.SendStream("progress", map[string]interface{}{
        "progress": progress,
        "message":  message,
    })
}

func (t *HTTPSSETransport) StartHeartbeat(ctx context.Context) {
    go func() {
        for {
            select {
            case <-ctx.Done():
                t.heartbeatTicker.Stop()
                return
            case <-t.heartbeatTicker.C:
                fmt.Fprintf(t.w, ":heartbeat\n\n")
                t.flusher.Flush()
            }
        }
    }()
}

func (t *HTTPSSETransport) Close() error {
    t.heartbeatTicker.Stop()
    fmt.Fprintf(t.w, "event: close\ndata: {}\n\n")
    t.flusher.Flush()
    return nil
}
```

### SSE Client Connection Example

```javascript
// JavaScript client
const eventSource = new EventSource('/mcp/stream', {
    headers: {
        'X-API-Key': 'your-api-key'
    }
});

eventSource.addEventListener('message', (event) => {
    const data = JSON.parse(event.data);
    console.log('Received:', data);
});

eventSource.addEventListener('progress', (event) => {
    const progress = JSON.parse(event.data);
    console.log('Progress:', progress.progress + '%');
});

eventSource.addEventListener('complete', (event) => {
    console.log('Stream complete');
    eventSource.close();
});

eventSource.addEventListener('error', (event) => {
    console.error('Stream error:', event);
});
```

### 3. WebSocket Implementation

For bidirectional real-time communication:

```go
// pkg/transport/websocket.go
package transport

import (
    "context"
    "encoding/json"
    "sync"

    "github.com/gorilla/websocket"
    "github.com/modelcontextprotocol/go-sdk/transport"
)

type WebSocketTransport struct {
    transport.Transport
    conn    *websocket.Conn
    mu      sync.Mutex
    sendCh  chan interface{}
    closeCh chan struct{}
}

func NewWebSocketTransport(conn *websocket.Conn) *WebSocketTransport {
    t := &WebSocketTransport{
        conn:    conn,
        sendCh:  make(chan interface{}, 100),
        closeCh: make(chan struct{}),
    }

    // Start send loop
    go t.sendLoop()

    return t
}

func (t *WebSocketTransport) Send(msg interface{}) error {
    select {
    case t.sendCh <- msg:
        return nil
    case <-t.closeCh:
        return websocket.ErrCloseSent
    }
}

func (t *WebSocketTransport) sendLoop() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case msg := <-t.sendCh:
            t.mu.Lock()
            if err := t.conn.WriteJSON(msg); err != nil {
                t.mu.Unlock()
                return
            }
            t.mu.Unlock()

        case <-ticker.C:
            t.mu.Lock()
            if err := t.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                t.mu.Unlock()
                return
            }
            t.mu.Unlock()

        case <-t.closeCh:
            return
        }
    }
}

func (t *WebSocketTransport) Receive() (json.RawMessage, error) {
    var msg json.RawMessage
    if err := t.conn.ReadJSON(&msg); err != nil {
        return nil, err
    }
    return msg, nil
}

func (t *WebSocketTransport) Close() error {
    close(t.closeCh)
    return t.conn.Close()
}
```

## Backpressure and Flow Control

Built-in backpressure management for all streaming transports:

```go
// pkg/streaming/backpressure.go
package streaming

import (
    "context"
    "sync/atomic"
    "time"
)

type BackpressureConfig struct {
    HighWaterMark   int64         // Pause threshold (bytes)
    LowWaterMark    int64         // Resume threshold (bytes)
    MaxBufferSize   int64         // Absolute maximum
    CheckInterval   time.Duration // Pressure check interval
}

type BackpressureManager struct {
    config        BackpressureConfig
    currentBytes  atomic.Int64
    currentItems  atomic.Int64
    paused        atomic.Bool
    pausedClients map[string]chan struct{}
    mu            sync.RWMutex
}

func NewBackpressureManager(config BackpressureConfig) *BackpressureManager {
    return &BackpressureManager{
        config:        config,
        pausedClients: make(map[string]chan struct{}),
    }
}

func (b *BackpressureManager) CheckPressure(clientID string, size int64) error {
    current := b.currentBytes.Load()

    if current+size > b.config.MaxBufferSize {
        return fmt.Errorf("buffer overflow: would exceed max size")
    }

    if current+size > b.config.HighWaterMark {
        // Apply backpressure
        b.paused.Store(true)

        // Create wait channel for this client
        b.mu.Lock()
        waitCh := make(chan struct{})
        b.pausedClients[clientID] = waitCh
        b.mu.Unlock()

        // Wait for pressure to reduce
        select {
        case <-waitCh:
            // Pressure relieved
            return nil
        case <-time.After(30 * time.Second):
            return fmt.Errorf("backpressure timeout")
        }
    }

    return nil
}

func (b *BackpressureManager) AddBytes(size int64) {
    b.currentBytes.Add(size)
    b.currentItems.Add(1)
}

func (b *BackpressureManager) RemoveBytes(size int64) {
    newBytes := b.currentBytes.Add(-size)
    b.currentItems.Add(-1)

    // Check if we can resume paused clients
    if b.paused.Load() && newBytes < b.config.LowWaterMark {
        b.paused.Store(false)

        b.mu.Lock()
        for clientID, ch := range b.pausedClients {
            close(ch)
            delete(b.pausedClients, clientID)
        }
        b.mu.Unlock()
    }
}

func (b *BackpressureManager) GetMetrics() map[string]interface{} {
    return map[string]interface{}{
        "currentBytes":  b.currentBytes.Load(),
        "currentItems":  b.currentItems.Load(),
        "isPaused":      b.paused.Load(),
        "pausedClients": len(b.pausedClients),
        "highWaterMark": b.config.HighWaterMark,
        "lowWaterMark":  b.config.LowWaterMark,
    }
}
```

## Streaming Decision Logic

The server automatically determines when to use streaming based on response size and configuration:

```go
// pkg/server/streaming.go
package server

import (
    "context"
    "encoding/json"
)

type StreamingConfig struct {
    Enabled         bool
    Threshold       int           // Number of items to trigger streaming
    ChunkSize       int           // Items per chunk
    FlushInterval   time.Duration // Force flush interval
}

func (s *MCPImmichServer) shouldStream(tool string, params json.RawMessage) bool {
    if !s.config.EnableStreaming {
        return false
    }

    // Check tool-specific logic
    switch tool {
    case "queryPhotos":
        var p QueryPhotosInput
        json.Unmarshal(params, &p)
        return p.Limit > s.config.StreamThreshold

    case "exportPhotos":
        return true // Always stream exports

    case "analyzePhotos":
        var p AnalyzePhotosInput
        json.Unmarshal(params, &p)
        return len(p.PhotoIds) > 10

    default:
        return false
    }
}

func (s *MCPImmichServer) streamResponse(ctx context.Context,
    transport StreamingTransport,
    dataChannel <-chan interface{}) error {

    buffer := make([]interface{}, 0, s.config.ChunkSize)
    flushTicker := time.NewTicker(s.config.FlushInterval)
    defer flushTicker.Stop()

    totalItems := 0
    startTime := time.Now()

    // Send start event
    transport.SendStream("start", map[string]interface{}{
        "timestamp": startTime,
        "streaming": true,
    })

    for {
        select {
        case item, ok := <-dataChannel:
            if !ok {
                // Channel closed, send final chunk
                if len(buffer) > 0 {
                    s.sendChunk(transport, buffer, totalItems)
                }

                // Send completion event
                transport.SendStream("complete", map[string]interface{}{
                    "totalItems": totalItems,
                    "duration":   time.Since(startTime).Seconds(),
                })
                return nil
            }

            buffer = append(buffer, item)
            totalItems++

            // Send chunk when buffer is full
            if len(buffer) >= s.config.ChunkSize {
                s.sendChunk(transport, buffer, totalItems)
                buffer = buffer[:0]
            }

        case <-flushTicker.C:
            // Periodic flush
            if len(buffer) > 0 {
                s.sendChunk(transport, buffer, totalItems)
                buffer = buffer[:0]
            }

        case <-ctx.Done():
            // Context cancelled
            transport.SendStream("error", map[string]interface{}{
                "error":   "stream_cancelled",
                "message": "Client disconnected",
            })
            return ctx.Err()
        }
    }
}

func (s *MCPImmichServer) sendChunk(transport StreamingTransport,
    items []interface{}, total int) error {

    return transport.SendStream("data", map[string]interface{}{
        "items":    items,
        "count":    len(items),
        "total":    total,
        "progress": calculateProgress(total),
    })
}
```

## Stream Lifecycle Management

### 1. Stream Initialization
```json
{
  "type": "stream_init",
  "streamId": "stream-123",
  "method": "queryPhotos",
  "expectedItems": 5000,
  "chunkSize": 100
}
```

### 2. Stream Progress
```json
{
  "type": "stream_progress",
  "streamId": "stream-123",
  "processedItems": 1000,
  "totalItems": 5000,
  "percentComplete": 20,
  "estimatedTimeRemaining": 24
}
```

### 3. Stream Data
```json
{
  "type": "stream_data",
  "streamId": "stream-123",
  "sequenceNumber": 10,
  "items": [...],
  "hasMore": true
}
```

### 4. Stream Completion
```json
{
  "type": "stream_complete",
  "streamId": "stream-123",
  "totalProcessed": 5000,
  "duration": 30,
  "success": true
}
```

### 5. Stream Error
```json
{
  "type": "stream_error",
  "streamId": "stream-123",
  "error": "Connection lost",
  "code": "STREAM_INTERRUPTED",
  "retryable": true,
  "resumeToken": "token-xyz"
}
```

## Error Recovery

### Resumable Streams

```go
type ResumableStream struct {
    StreamID     string
    LastSequence int64
    Checkpoint   []byte
    ExpiresAt    time.Time
}

func (r *ResumableStream) Resume(fromSequence int64) (io.ReadCloser, error) {
    // Validate resume token
    if time.Now().After(r.ExpiresAt) {
        return nil, errors.New("resume token expired")
    }

    // Skip to checkpoint
    reader := r.createReader()
    if err := r.seekToSequence(reader, fromSequence); err != nil {
        return nil, err
    }

    return reader, nil
}
```

### Retry Strategy
```go
type RetryConfig struct {
    MaxAttempts     int
    InitialBackoff  time.Duration
    MaxBackoff      time.Duration
    BackoffMultiplier float64
}

func (r *RetryConfig) RetryWithBackoff(fn func() error) error {
    backoff := r.InitialBackoff

    for attempt := 0; attempt < r.MaxAttempts; attempt++ {
        err := fn()
        if err == nil {
            return nil
        }

        if !isRetryable(err) {
            return err
        }

        time.Sleep(backoff)
        backoff = time.Duration(float64(backoff) * r.BackoffMultiplier)
        if backoff > r.MaxBackoff {
            backoff = r.MaxBackoff
        }
    }

    return fmt.Errorf("max retry attempts exceeded")
}
```

## Performance Optimizations

### 1. Buffer Pool
```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 64*1024) // 64KB buffers
    },
}

func getBuffer() []byte {
    return bufferPool.Get().([]byte)
}

func putBuffer(buf []byte) {
    bufferPool.Put(buf)
}
```

### 2. Zero-Copy Streaming
```go
func zeroCopyStream(src io.Reader, dst io.Writer) error {
    // Use sendfile syscall on Linux
    if tc, ok := src.(*net.TCPConn); ok {
        if tw, ok := dst.(*net.TCPConn); ok {
            return sendfile(tw, tc)
        }
    }

    // Fallback to io.Copy
    _, err := io.Copy(dst, src)
    return err
}
```

### 3. Compression
```go
type CompressedStream struct {
    writer io.Writer
    compressor io.WriteCloser
}

func NewCompressedStream(w io.Writer, algorithm string) (*CompressedStream, error) {
    var compressor io.WriteCloser

    switch algorithm {
    case "gzip":
        compressor = gzip.NewWriter(w)
    case "br":
        compressor = brotli.NewWriter(w)
    case "zstd":
        compressor = zstd.NewWriter(w)
    default:
        return nil, errors.New("unsupported compression")
    }

    return &CompressedStream{
        writer: w,
        compressor: compressor,
    }, nil
}
```

## Monitoring and Metrics

### Key Metrics
```go
type StreamMetrics struct {
    ActiveStreams      atomic.Int64
    TotalBytesStreamed atomic.Int64
    StreamErrors       atomic.Int64
    AverageLatency     atomic.Int64
    BackpressureEvents atomic.Int64
}

func (m *StreamMetrics) RecordStream(bytes int64, latency time.Duration) {
    m.TotalBytesStreamed.Add(bytes)
    m.AverageLatency.Store(int64(latency))
}
```

### Health Check Endpoint
```json
GET /health/streaming

{
  "status": "healthy",
  "activeStreams": 42,
  "maxStreams": 100,
  "bufferUtilization": 0.65,
  "averageLatencyMs": 45,
  "errorsLast5Min": 2
}
```

## Client Integration Examples

### TypeScript/JavaScript MCP Client

```typescript
import { Client } from '@modelcontextprotocol/client';

// Initialize MCP client with streaming support
const client = new Client({
    transport: {
        type: 'sse',  // or 'websocket'
        url: 'http://localhost:8080/mcp/stream',
        headers: {
            'X-API-Key': 'your-api-key'  // if auth enabled
        }
    }
});

// Connect and initialize
await client.connect();
await client.initialize();

// List available tools
const tools = await client.listTools();
console.log('Available tools:', tools);

// Call a tool with streaming
const result = await client.callTool('queryPhotos', {
    query: 'sunset beach',
    limit: 1000  // Will trigger streaming
});

// Handle streaming response
if (result.streaming) {
    for await (const chunk of result.stream) {
        console.log(`Received ${chunk.items.length} photos`);
        processPhotos(chunk.items);
    }
}
```

### Go Client Using MCP SDK

```go
package main

import (
    "context"
    "log"

    mcp "github.com/modelcontextprotocol/go-sdk"
    "github.com/modelcontextprotocol/go-sdk/transport"
)

func main() {
    // Create SSE transport
    sseTransport := transport.NewSSETransport(
        "http://localhost:8080/mcp/stream",
        map[string]string{
            "X-API-Key": "your-api-key",
        },
    )

    // Create MCP client
    client := mcp.NewClient(
        mcp.WithTransport(sseTransport),
    )

    ctx := context.Background()

    // Initialize connection
    if err := client.Initialize(ctx); err != nil {
        log.Fatal("Failed to initialize:", err)
    }

    // List tools
    tools, err := client.ListTools(ctx)
    if err != nil {
        log.Fatal("Failed to list tools:", err)
    }

    log.Printf("Available tools: %+v", tools)

    // Call tool with potential streaming
    result, err := client.CallTool(ctx, "queryPhotos", map[string]interface{}{
        "query": "sunset beach",
        "limit": 1000,
    })

    if err != nil {
        log.Fatal("Tool call failed:", err)
    }

    // Handle result (streaming handled automatically by transport)
    log.Printf("Result: %+v", result)
}
```

### Python Client

```python
import asyncio
import json
import aiohttp
from aiohttp_sse_client import client as sse_client

class MCPStreamingClient:
    def __init__(self, base_url, api_key=None):
        self.base_url = base_url
        self.headers = {'X-API-Key': api_key} if api_key else {}

    async def stream_query(self, query_params):
        """Stream photo query results via SSE"""

        async with aiohttp.ClientSession() as session:
            # Send initial request
            async with session.post(
                f"{self.base_url}/mcp",
                headers={**self.headers, 'Accept': 'text/event-stream'},
                json={
                    "jsonrpc": "2.0",
                    "method": "tools/call",
                    "params": {
                        "name": "queryPhotos",
                        "arguments": query_params
                    },
                    "id": 1
                }
            ) as response:
                # Check if streaming
                if response.headers.get('Content-Type') == 'text/event-stream':
                    async for event in sse_client.EventSource(response):
                        if event.event == 'data':
                            data = json.loads(event.data)
                            yield data['items']
                        elif event.event == 'complete':
                            break
                else:
                    # Non-streaming response
                    result = await response.json()
                    yield result['result']['content']

# Usage
async def main():
    client = MCPStreamingClient(
        'http://localhost:8080',
        api_key='your-api-key'
    )

    async for photos in client.stream_query({
        'query': 'sunset',
        'limit': 1000
    }):
        print(f"Received {len(photos)} photos")
        # Process photos batch

if __name__ == '__main__':
    asyncio.run(main())
```

## Configuration and Tuning

### Streaming Configuration Options

```yaml
# config.yaml
streaming:
  enabled: true

  # Thresholds
  auto_stream_threshold: 500    # Items before auto-streaming
  chunk_size: 100               # Items per chunk

  # Timeouts
  stream_timeout: "5m"          # Maximum stream duration
  idle_timeout: "30s"           # Idle connection timeout
  flush_interval: "100ms"       # Force flush interval

  # Backpressure
  high_water_mark: 10485760     # 10MB - pause threshold
  low_water_mark: 5242880       # 5MB - resume threshold
  max_buffer_size: 52428800     # 50MB - absolute maximum

  # Transport preferences
  preferred_transport: "sse"    # sse, chunked, websocket
  enable_compression: true       # gzip compression for large payloads

  # Performance
  max_concurrent_streams: 100
  stream_buffer_size: 65536     # Per-stream buffer
```

### Environment Variables

```bash
# Override configuration via environment
export MCP_STREAMING_ENABLED=true
export MCP_STREAMING_AUTO_STREAM_THRESHOLD=1000
export MCP_STREAMING_CHUNK_SIZE=200
export MCP_STREAMING_HIGH_WATER_MARK=20971520
```

### Performance Monitoring

```go
// Streaming metrics exposed at /metrics
streaming_active_connections{transport="sse"} 42
streaming_bytes_sent_total{transport="sse"} 1234567890
streaming_chunks_sent_total{tool="queryPhotos"} 5678
streaming_backpressure_events_total 23
streaming_stream_duration_seconds{quantile="0.99"} 4.2
```

## Testing Streaming

### Manual Testing with curl

```bash
# Test SSE streaming
curl -N -H "Accept: text/event-stream" \
     -H "X-API-Key: your-key" \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"queryPhotos","arguments":{"query":"sunset","limit":1000}},"id":1}' \
     http://localhost:8080/mcp/stream

# Test WebSocket with wscat
wscat -c ws://localhost:8080/mcp/ws \
      -H "X-API-Key: your-key" \
      -x '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"queryPhotos","arguments":{"limit":1000}},"id":1}'
```

### Load Testing Script

```go
// test/load_test.go
package test

import (
    "context"
    "sync"
    "testing"
    "time"
)

func TestStreamingLoad(t *testing.T) {
    ctx := context.Background()

    // Test configuration
    numClients := 100
    numRequests := 10
    itemsPerRequest := 1000

    var wg sync.WaitGroup
    errors := make([]error, 0)
    var mu sync.Mutex

    start := time.Now()

    for i := 0; i < numClients; i++ {
        wg.Add(1)
        go func(clientID int) {
            defer wg.Done()

            client := createTestClient()

            for j := 0; j < numRequests; j++ {
                result, err := client.CallTool(ctx, "queryPhotos", map[string]interface{}{
                    "limit": itemsPerRequest,
                })

                if err != nil {
                    mu.Lock()
                    errors = append(errors, err)
                    mu.Unlock()
                    return
                }

                // Verify result
                if result == nil {
                    t.Errorf("Client %d: nil result", clientID)
                }
            }
        }(i)
    }

    wg.Wait()
    duration := time.Since(start)

    // Report results
    t.Logf("Load test completed:")
    t.Logf("  Duration: %v", duration)
    t.Logf("  Total requests: %d", numClients*numRequests)
    t.Logf("  Requests/sec: %.2f", float64(numClients*numRequests)/duration.Seconds())
    t.Logf("  Errors: %d", len(errors))

    if len(errors) > 0 {
        t.Fatalf("Load test had %d errors", len(errors))
    }
}
```