package shutdown

import (
	"context"
	"errors"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
)

// mockShutdown implements the Shutdown interface for testing
type mockShutdown struct {
	shutdownFunc   func(context.Context) error
	shutdownCalled bool
	mu             sync.Mutex
}

func (m *mockShutdown) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownCalled = true
	if m.shutdownFunc != nil {
		return m.shutdownFunc(ctx)
	}
	return nil
}

func (m *mockShutdown) wasShutdownCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shutdownCalled
}

func TestAddShutdownHook(t *testing.T) {
	tests := []struct {
		name        string
		shutdowns   []Shutdown
		signal      syscall.Signal
		timeout     time.Duration
		expectFlush bool
	}{
		{
			name: "single shutdown handler success",
			shutdowns: []Shutdown{
				&mockShutdown{},
			},
			signal:      syscall.SIGTERM,
			timeout:     5 * time.Second,
			expectFlush: true,
		},
		{
			name: "multiple shutdown handlers success",
			shutdowns: []Shutdown{
				&mockShutdown{},
				&mockShutdown{},
				&mockShutdown{},
			},
			signal:      syscall.SIGINT,
			timeout:     5 * time.Second,
			expectFlush: true,
		},
		{
			name: "shutdown handler with error",
			shutdowns: []Shutdown{
				&mockShutdown{
					shutdownFunc: func(ctx context.Context) error {
						return errors.New("shutdown error")
					},
				},
			},
			signal:      syscall.SIGTERM,
			timeout:     5 * time.Second,
			expectFlush: true,
		},
		{
			name:        "no shutdown handlers",
			shutdowns:   []Shutdown{},
			signal:      syscall.SIGTERM,
			timeout:     5 * time.Second,
			expectFlush: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create context with cancel
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create logger
			logger := klog.NewKlogr()

			// Run AddShutdownHook in a goroutine
			done := make(chan struct{})
			go func() {
				defer close(done)
				AddShutdownHook(ctx, logger, tt.timeout, tt.shutdowns...)
			}()

			// Give the goroutine time to start
			time.Sleep(50 * time.Millisecond)

			// Cancel the context to simulate signal
			cancel()

			// Wait for completion with timeout
			select {
			case <-done:
				// Success
			case <-time.After(1 * time.Second):
				t.Fatal("AddShutdownHook did not complete in time")
			}

			// Verify all shutdown handlers were called
			for i, s := range tt.shutdowns {
				if mock, ok := s.(*mockShutdown); ok {
					assert.True(t, mock.wasShutdownCalled(), "shutdown handler %d was not called", i)
				}
			}
		})
	}
}

func TestAddShutdownHookTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a shutdown handler that takes longer than timeout
	slowShutdown := &mockShutdown{
		shutdownFunc: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Second):
				return nil
			}
		},
	}

	logger := klog.NewKlogr()

	// Run with very short timeout
	done := make(chan struct{})
	go func() {
		defer close(done)
		AddShutdownHook(ctx, logger, 50*time.Millisecond, slowShutdown)
	}()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for completion
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("AddShutdownHook did not complete in time")
	}

	// Verify shutdown was called
	assert.True(t, slowShutdown.wasShutdownCalled())
}

func TestAddShutdownHookMultipleErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create shutdown handlers with errors
	shutdowns := []Shutdown{
		&mockShutdown{
			shutdownFunc: func(ctx context.Context) error {
				return errors.New("error 1")
			},
		},
		&mockShutdown{
			shutdownFunc: func(ctx context.Context) error {
				return errors.New("error 2")
			},
		},
		&mockShutdown{}, // This one succeeds
	}

	logger := klog.NewKlogr()

	// Run shutdown hook
	done := make(chan struct{})
	go func() {
		defer close(done)
		AddShutdownHook(ctx, logger, 5*time.Second, shutdowns...)
	}()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for completion
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("AddShutdownHook did not complete in time")
	}

	// Verify all shutdown handlers were called despite errors
	for i, s := range shutdowns {
		mock := s.(*mockShutdown)
		assert.True(t, mock.wasShutdownCalled(), "shutdown handler %d was not called", i)
	}
}

func TestShutdownInterface(t *testing.T) {
	// Test that mockShutdown properly implements Shutdown interface
	var _ Shutdown = (*mockShutdown)(nil)

	// Test successful shutdown
	mock := &mockShutdown{}
	err := mock.Shutdown(context.Background())
	require.NoError(t, err)
	assert.True(t, mock.wasShutdownCalled())

	// Test shutdown with error
	mockWithError := &mockShutdown{
		shutdownFunc: func(ctx context.Context) error {
			return errors.New("shutdown failed")
		},
	}
	err = mockWithError.Shutdown(context.Background())
	require.Error(t, err)
	assert.Equal(t, "shutdown failed", err.Error())
	assert.True(t, mockWithError.wasShutdownCalled())

	// Test shutdown with canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mockWithContext := &mockShutdown{
		shutdownFunc: func(ctx context.Context) error {
			return ctx.Err()
		},
	}
	err = mockWithContext.Shutdown(ctx)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}
