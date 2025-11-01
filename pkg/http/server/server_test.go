package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
)

func TestNewHTTPServerArgs(t *testing.T) {
	args := NewHTTPServerArgs()

	assert.NotNil(t, args)
	assert.Equal(t, ":443", args.Addr)
	assert.Equal(t, "unix:///spiffe-workload-api/spire-agent.sock", args.SpireSocketPath)
}
func TestHTTPServerLivenessHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET request returns 200",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			expectedBody:   "",
		},
		{
			name:           "POST request returns 200",
			method:         http.MethodPost,
			expectedStatus: http.StatusOK,
			expectedBody:   "",
		},
		{
			name:           "HEAD request returns 200",
			method:         http.MethodHead,
			expectedStatus: http.StatusOK,
			expectedBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create server instance
			server := &HTTPServer{
				healthy: atomic.NewBool(true),
			}

			// Create request
			req := httptest.NewRequest(tt.method, "/-/-/liveness", nil)
			w := httptest.NewRecorder()

			// Call handler directly
			server.livenessHandler(w, req)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestHTTPServerReadinessHandler(t *testing.T) {
	tests := []struct {
		name           string
		healthy        bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "healthy server returns 200",
			healthy:        true,
			expectedStatus: http.StatusOK,
			expectedBody:   "",
		},
		{
			name:           "unhealthy server returns 503",
			healthy:        false,
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create server instance
			server := &HTTPServer{
				healthy: atomic.NewBool(tt.healthy),
			}

			// Create request
			req := httptest.NewRequest(http.MethodGet, "/-/-/readiness", nil)
			w := httptest.NewRecorder()

			// Call handler directly
			server.readinessHandler(w, req)

			// Assert response
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedBody, w.Body.String())
		})
	}
}

func TestHTTPServerHealthStateTransitions(t *testing.T) {
	// Test that health state transitions work correctly
	server := &HTTPServer{
		healthy: atomic.NewBool(true),
	}

	// Initially healthy
	req := httptest.NewRequest(http.MethodGet, "/-/-/readiness", nil)
	w := httptest.NewRecorder()
	server.readinessHandler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Mark as unhealthy
	server.healthy.Store(false)
	w = httptest.NewRecorder()
	server.readinessHandler(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	// Mark as healthy again
	server.healthy.Store(true)
	w = httptest.NewRecorder()
	server.readinessHandler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Liveness should always return 200
	req = httptest.NewRequest(http.MethodGet, "/-/-/liveness", nil)
	w = httptest.NewRecorder()
	server.livenessHandler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Even when unhealthy
	server.healthy.Store(false)
	w = httptest.NewRecorder()
	server.livenessHandler(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPServerConcurrentHealthChecks(t *testing.T) {
	// Test concurrent access to health endpoints
	server := &HTTPServer{
		healthy: atomic.NewBool(true),
	}

	// Run multiple goroutines accessing health endpoints
	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// Alternate between liveness and readiness
			endpoint := "/-/-/liveness"
			if i%2 == 0 {
				endpoint = "/-/-/readiness"
			}

			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			w := httptest.NewRecorder()

			if endpoint == "/-/-/liveness" {
				server.livenessHandler(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				server.readinessHandler(w, req)
				// Could be either 200 or 503 depending on timing
				assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, w.Code)
			}
		}(i)

		// Some goroutines toggle health state
		if i%10 == 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				server.healthy.Store(false)
				time.Sleep(1 * time.Millisecond)
				server.healthy.Store(true)
			}()
		}
	}

	wg.Wait()
}

func TestHTTPServerShutdown(t *testing.T) {
	tests := []struct {
		name             string
		setupServer      func() *HTTPServer
		shutdownContext  func() (context.Context, context.CancelFunc)
		expectedHealthy  bool
		expectError      bool
		expectKeepAlives bool
	}{
		{
			name: "successful shutdown with timeout context",
			setupServer: func() *HTTPServer {
				return &HTTPServer{
					server:  &http.Server{},
					healthy: atomic.NewBool(true),
				}
			},
			shutdownContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 5*time.Second)
			},
			expectedHealthy:  false,
			expectError:      false,
			expectKeepAlives: false,
		},
		{
			name: "shutdown with cancelled context",
			setupServer: func() *HTTPServer {
				return &HTTPServer{
					server:  &http.Server{},
					healthy: atomic.NewBool(true),
				}
			},
			shutdownContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				return ctx, cancel
			},
			expectedHealthy:  false,
			expectError:      false, // Server.Shutdown handles cancelled context gracefully
			expectKeepAlives: false,
		},
		{
			name: "shutdown already unhealthy server",
			setupServer: func() *HTTPServer {
				return &HTTPServer{
					server:  &http.Server{},
					healthy: atomic.NewBool(false), // Already unhealthy
				}
			},
			shutdownContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 5*time.Second)
			},
			expectedHealthy:  false,
			expectError:      false,
			expectKeepAlives: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			server := tt.setupServer()
			ctx, cancel := tt.shutdownContext()
			defer cancel()

			// Execute
			err := server.Shutdown(ctx)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedHealthy, server.healthy.Load())
		})
	}
}

func TestHTTPServerShutdownWithActiveConnections(t *testing.T) {
	// Create a test server with handler
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow handler
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	server := &HTTPServer{
		server: &http.Server{
			Handler: mux,
		},
		healthy: atomic.NewBool(true),
	}

	// Start test server
	testServer := httptest.NewServer(server.server.Handler)
	defer testServer.Close()

	// Start a request that will be in-flight during shutdown
	go func() {
		_, err := http.Get(testServer.URL + "/slow")
		if err != nil {
			t.Error(err)
			return
		}
	}()

	// Give the request time to start
	time.Sleep(10 * time.Millisecond)

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	assert.NoError(t, err)
	assert.False(t, server.healthy.Load())
}

func TestHTTPServerShutdownIdempotent(t *testing.T) {
	// Test that calling Shutdown multiple times is safe
	server := &HTTPServer{
		server:  &http.Server{},
		healthy: atomic.NewBool(true),
	}

	ctx := context.Background()

	// First shutdown
	err := server.Shutdown(ctx)
	assert.NoError(t, err)
	assert.False(t, server.healthy.Load())

	// Second shutdown
	err = server.Shutdown(ctx)
	assert.NoError(t, err)
	assert.False(t, server.healthy.Load())

	// Third shutdown
	err = server.Shutdown(ctx)
	assert.NoError(t, err)
	assert.False(t, server.healthy.Load())
}

func TestHTTPServerShutdownConcurrent(t *testing.T) {
	// Test concurrent shutdown calls
	server := &HTTPServer{
		server:  &http.Server{},
		healthy: atomic.NewBool(true),
	}

	var wg sync.WaitGroup
	numGoroutines := 10
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			err := server.Shutdown(ctx)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		assert.NoError(t, err)
	}

	// Server should be unhealthy
	assert.False(t, server.healthy.Load())
}

func TestHTTPServerShutdownHealthTransition(t *testing.T) {
	// Test that health status changes immediately during the shutdown
	server := &HTTPServer{
		server:  &http.Server{},
		healthy: atomic.NewBool(true),
	}

	// Verify initial state
	assert.True(t, server.healthy.Load())

	// Start shutdown in goroutine
	shutdownStarted := make(chan struct{})
	shutdownComplete := make(chan struct{})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		close(shutdownStarted)
		_ = server.Shutdown(ctx)
		close(shutdownComplete)
	}()

	// Wait for shutdown to start
	<-shutdownStarted

	// Health should be false immediately
	assert.False(t, server.healthy.Load())

	// Wait for shutdown to complete
	<-shutdownComplete

	// Health should still be false
	assert.False(t, server.healthy.Load())
}
