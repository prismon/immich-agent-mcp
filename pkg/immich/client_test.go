package immich

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestClientPingSuccess(t *testing.T) {
	t.Parallel()

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		assert.Equal(t, "/api/server-info/ping", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second)
	err := client.Ping(context.Background())

	assert.NoError(t, err)
	assert.True(t, called)
}

func TestClientPingError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second)
	err := client.Ping(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ping failed with status")
}

func TestClientRequestSendsPayload(t *testing.T) {
	t.Parallel()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		assert.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{\"ok\":true}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second)

	var result struct {
		OK bool `json:"ok"`
	}

	err := client.post(context.Background(), server.URL+"/test", map[string]string{"hello": "world"}, &result)

	assert.NoError(t, err)
	assert.True(t, result.OK)
	assert.JSONEq(t, "{\"hello\":\"world\"}", string(receivedBody))
}

func TestClientRequestErrorStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second)

	err := client.get(context.Background(), server.URL+"/bad", &struct{}{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status=400")
	assert.Contains(t, err.Error(), "bad request")
}

func TestClientRequestDecodeError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key", time.Second)

	var result struct{}
	err := client.get(context.Background(), server.URL+"/decode", &result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode response")
}
