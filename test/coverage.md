# Test Coverage Strategy

## Coverage Requirements

### Target: 80% Code Coverage

The project must maintain a minimum of 80% code coverage across all packages.

## Testing Approach

### 1. Unit Tests (60% of coverage)
Test individual functions and methods in isolation.

```bash
# Run unit tests with coverage
go test -coverprofile=coverage.out -covermode=atomic ./pkg/...
```

### 2. Integration Tests (15% of coverage)
Test component interactions with mocked external dependencies.

```bash
# Run integration tests
go test -tags=integration -coverprofile=integration.out ./test/integration/...
```

### 3. Smoke Tests (5% of coverage)
Basic functionality tests against real Immich instance.

```bash
# Run smoke tests (requires test environment)
go test -tags=smoke -coverprofile=smoke.out ./test/
```

## Package Coverage Targets

| Package | Min Coverage | Priority | Notes |
|---------|-------------|----------|--------|
| `pkg/server` | 85% | High | Core MCP server logic |
| `pkg/tools` | 90% | High | All tool implementations |
| `pkg/immich` | 85% | High | Immich API client |
| `pkg/auth` | 80% | Medium | Authentication providers |
| `pkg/cache` | 75% | Medium | Caching layer |
| `pkg/config` | 70% | Low | Configuration loading |
| `cmd/mcp-immich` | 60% | Low | Main entry point |

## Test Structure

### Tool Tests Template

Each tool should have comprehensive tests:

```go
// pkg/tools/query_photos_test.go
package tools

import (
    "context"
    "testing"

    "github.com/golang/mock/gomock"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestQueryPhotosTool_Execute(t *testing.T) {
    tests := []struct {
        name      string
        input     string
        mockSetup func(*MockImmichClient)
        wantErr   bool
        validate  func(*testing.T, *protocol.ToolResult)
    }{
        {
            name:  "successful query",
            input: `{"limit": 10}`,
            mockSetup: func(m *MockImmichClient) {
                m.EXPECT().QueryPhotos(gomock.Any(), gomock.Any()).
                    Return(&PhotoResults{TotalCount: 10}, nil)
            },
            wantErr: false,
            validate: func(t *testing.T, result *protocol.ToolResult) {
                assert.NotNil(t, result)
                assert.Len(t, result.Content, 2)
            },
        },
        {
            name:    "invalid input",
            input:   `{"limit": "invalid"}`,
            wantErr: true,
        },
        {
            name:  "immich error",
            input: `{"limit": 10}`,
            mockSetup: func(m *MockImmichClient) {
                m.EXPECT().QueryPhotos(gomock.Any(), gomock.Any()).
                    Return(nil, errors.New("API error"))
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            defer ctrl.Finish()

            mockClient := NewMockImmichClient(ctrl)
            if tt.mockSetup != nil {
                tt.mockSetup(mockClient)
            }

            tool := &QueryPhotosTool{immich: mockClient}
            result, err := tool.Execute(context.Background(),
                json.RawMessage(tt.input))

            if tt.wantErr {
                assert.Error(t, err)
                return
            }

            require.NoError(t, err)
            if tt.validate != nil {
                tt.validate(t, result)
            }
        })
    }
}
```

## Coverage Measurement

### CI Pipeline

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run tests with coverage
        run: |
          go test -coverprofile=coverage.out -covermode=atomic ./...
          go tool cover -func=coverage.out

      - name: Check coverage threshold
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Total coverage: $COVERAGE%"
          if (( $(echo "$COVERAGE < 80" | bc -l) )); then
            echo "Coverage is below 80% threshold"
            exit 1
          fi

      - name: Upload coverage to Codecov
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

### Local Coverage Commands

```bash
# Run all tests with coverage
make test-coverage

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html

# Check if coverage meets threshold
./scripts/check-coverage.sh 80
```

### Makefile Targets

```makefile
# Makefile
.PHONY: test test-coverage test-unit test-integration test-smoke coverage-html

test:
	go test -v ./...

test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

test-unit:
	go test -v -short ./pkg/...

test-integration:
	go test -v -tags=integration ./test/integration/...

test-smoke:
	go test -v -tags=smoke ./test/

coverage-html: test-coverage
	go tool cover -html=coverage.out -o coverage.html
	open coverage.html

check-coverage: test-coverage
	@COVERAGE=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	echo "Total coverage: $$COVERAGE%"; \
	if [ $$(echo "$$COVERAGE < 80" | bc) -eq 1 ]; then \
		echo "❌ Coverage is below 80% threshold"; \
		exit 1; \
	else \
		echo "✅ Coverage meets 80% threshold"; \
	fi
```

## Mock Generation

```bash
# Install mockgen
go install github.com/golang/mock/mockgen@latest

# Generate mocks for interfaces
mockgen -source=pkg/immich/client.go -destination=pkg/immich/mock_client.go -package=immich
mockgen -source=pkg/cache/cache.go -destination=pkg/cache/mock_cache.go -package=cache
```

## Test Data Management

### Test Fixtures
```
test/
├── fixtures/
│   ├── photos.json       # Sample photo data
│   ├── albums.json       # Sample album data
│   ├── broken_files.json # Sample broken file data
│   └── libraries.json    # Sample library data
├── smoke_test.go
├── coverage.md
└── testdata/
    └── config.yaml       # Test configuration
```

## Coverage Exclusions

Some files may be excluded from coverage:

```go
// .coveragerc or go test flags
// Exclude generated files
**/mock_*.go
**/generated.go

// Exclude main.go (minimal logic)
cmd/mcp-immich/main.go

// Exclude pure interfaces
**/interfaces.go
```

## Best Practices

1. **Test Naming**: Use descriptive test names following `Test<Function>_<Scenario>`
2. **Table-Driven Tests**: Use table-driven tests for comprehensive coverage
3. **Mock External Dependencies**: Never call real Immich API in unit tests
4. **Test Error Cases**: Always test error paths and edge cases
5. **Parallel Tests**: Use `t.Parallel()` where safe to speed up tests
6. **Cleanup**: Always cleanup test resources (defer cleanup functions)
7. **Assertions**: Use testify assertions for clear test failures

## Coverage Reports

### Console Output
```
github.com/yourusername/mcp-immich/pkg/server    85.2%
github.com/yourusername/mcp-immich/pkg/tools     91.3%
github.com/yourusername/mcp-immich/pkg/immich    86.7%
github.com/yourusername/mcp-immich/pkg/auth      82.1%
github.com/yourusername/mcp-immich/pkg/cache     78.9%
github.com/yourusername/mcp-immich/cmd/mcp-immich 65.0%
total:                                            84.3%
```

### Coverage Badge
Add to README:
```markdown
[![Coverage](https://codecov.io/gh/yourusername/mcp-immich/branch/main/graph/badge.svg)](https://codecov.io/gh/yourusername/mcp-immich)
```

## Continuous Monitoring

1. **PR Checks**: Block PRs that reduce coverage below 80%
2. **Coverage Trends**: Track coverage over time with Codecov/Coveralls
3. **Weekly Reports**: Generate coverage reports in CI
4. **Team Goals**: Set quarterly goals to improve coverage

## Testing Checklist

Before each release, ensure:

- [ ] All unit tests pass
- [ ] Integration tests pass
- [ ] Smoke tests pass against test Immich instance
- [ ] Coverage is ≥80% overall
- [ ] All packages meet minimum coverage targets
- [ ] No critical paths untested
- [ ] Error handling thoroughly tested
- [ ] Edge cases covered
- [ ] Performance benchmarks pass