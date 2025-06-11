package tserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"
)

// TestApp is a test implementation of the App interface used for testing purposes.
// It simulates application behavior with configurable delays and error conditions.
type TestApp struct {
	name           string        // Application name for identification
	runDuration    time.Duration // How long the app should run before stopping
	runErr         error         // Error to return from Run method (if any)
	shutdownErr    error         // Error to return from Shutdown method (if any)
	runCalled      bool          // Flag to track if Run was called
	shutdownCalled bool          // Flag to track if Shutdown was called
	mu             sync.Mutex    // Mutex for thread-safe access to flags
}

// NewTestApp creates a new TestApp instance with specified parameters.
//
// Parameters:
//   - name: Human-readable name for the application
//   - runDuration: How long the Run method should block before returning
//   - runErr: Error to return from Run method (nil for success)
//   - shutdownErr: Error to return from Shutdown method (nil for success)
//
// Returns:
//   - *TestApp: Configured test application instance
func NewTestApp(name string, runDuration time.Duration, runErr, shutdownErr error) *TestApp {
	return &TestApp{
		name:           name,
		runDuration:    runDuration,
		runErr:         runErr,
		shutdownErr:    shutdownErr,
		runCalled:      false,
		shutdownCalled: false,
	}
}

// Run implements the App interface by simulating application execution.
// It blocks for the configured duration or until context cancellation.
func (t *TestApp) Run(ctx context.Context) error {
	t.mu.Lock()
	t.runCalled = true
	t.mu.Unlock()

	// If no duration specified, wait for context cancellation
	if t.runDuration == 0 {
		<-ctx.Done()
		return t.runErr
	}

	// Wait for either timeout or context cancellation
	select {
	case <-time.After(t.runDuration):
		return t.runErr
	case <-ctx.Done():
		return t.runErr
	}
}

// Shutdown implements the App interface by simulating graceful shutdown.
// It marks the shutdown as called and returns any configured error.
func (t *TestApp) Shutdown(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.shutdownCalled = true
	return t.shutdownErr
}

// IsRunCalled returns whether the Run method has been called.
// This method is thread-safe.
func (t *TestApp) IsRunCalled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.runCalled
}

// IsShutdownCalled returns whether the Shutdown method has been called.
// This method is thread-safe.
func (t *TestApp) IsShutdownCalled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.shutdownCalled
}

// TestHook is a test implementation of the Hook interface used for testing.
// It tracks execution and can simulate hook failures.
type TestHook struct {
	name     string     // Hook name for identification
	err      error      // Error to return when executed (nil for success)
	executed bool       // Flag to track if hook was executed
	mu       sync.Mutex // Mutex for thread-safe access
}

// NewTestHook creates a new TestHook instance.
//
// Parameters:
//   - name: Human-readable name for the hook
//   - err: Error to return when executed (nil for success)
//
// Returns:
//   - *TestHook: Configured test hook instance
func NewTestHook(name string, err error) *TestHook {
	return &TestHook{
		name:     name,
		err:      err,
		executed: false,
	}
}

// Run implements the Hook interface by marking execution and returning configured error.
func (h *TestHook) Run(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.executed = true
	return h.err
}

// IsExecuted returns whether the hook has been executed.
// This method is thread-safe.
func (h *TestHook) IsExecuted() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.executed
}

// Test_NewServer tests the creation of a new server instance.
// It verifies that the server is properly initialized with default values.
func Test_NewServer(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer(ctx)

	// Server creation should succeed
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Server should not be nil
	if server == nil {
		t.Fatal("Server should not be nil")
	}

	// Server should not be running initially
	if server.IsRunning() {
		t.Error("New server should not be running")
	}

	// Apps slice should be initialized and empty
	apps := server.GetApps()
	if len(apps) != 0 {
		t.Errorf("Expected 0 apps, got %d", len(apps))
	}
}

// Test_NewServerWithApp tests server creation with a single application.
func Test_NewServerWithApp(t *testing.T) {
	ctx := context.Background()
	app := NewTestApp("test-app", 100*time.Millisecond, nil, nil)

	server, err := NewServer(ctx, WithApp(app))
	if err != nil {
		t.Fatalf("Failed to create server with app: %v", err)
	}

	// Should have exactly one app
	apps := server.GetApps()
	if len(apps) != 1 {
		t.Errorf("Expected 1 app, got %d", len(apps))
	}
}

// Test_NewServerWithApps tests server creation with multiple applications.
func Test_NewServerWithApps(t *testing.T) {
	ctx := context.Background()
	app1 := NewTestApp("app1", 100*time.Millisecond, nil, nil)
	app2 := NewTestApp("app2", 100*time.Millisecond, nil, nil)
	app3 := NewTestApp("app3", 100*time.Millisecond, nil, nil)

	server, err := NewServer(ctx, WithApps(app1, app2, app3))
	if err != nil {
		t.Fatalf("Failed to create server with apps: %v", err)
	}

	// Should have exactly three apps
	apps := server.GetApps()
	if len(apps) != 3 {
		t.Errorf("Expected 3 apps, got %d", len(apps))
	}
}

// Test_ServerStartStop tests basic server start and stop functionality.
func Test_ServerStartStop(t *testing.T) {
	ctx := context.Background()
	app := NewTestApp("test-app", 0, nil, nil) // App runs until context is cancelled

	server, err := NewServer(ctx, WithApp(app))
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start the server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Server should be running
	if !server.IsRunning() {
		t.Error("Server should be running after start")
	}

	// Wait a bit for app to start
	time.Sleep(50 * time.Millisecond)

	// App's Run method should have been called
	if !app.IsRunCalled() {
		t.Error("App Run method should have been called")
	}

	// Stop the server
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// Server should not be running
	if server.IsRunning() {
		t.Error("Server should not be running after stop")
	}

	// App's Shutdown method should have been called
	if !app.IsShutdownCalled() {
		t.Error("App Shutdown method should have been called")
	}
}

// Test_ServerMultipleApps tests server functionality with multiple applications.
func Test_ServerMultipleApps(t *testing.T) {
	ctx := context.Background()

	// Create multiple test applications
	apps := []*TestApp{
		NewTestApp("app1", 0, nil, nil),
		NewTestApp("app2", 0, nil, nil),
		NewTestApp("app3", 0, nil, nil),
	}

	// Convert to interface slice
	iApps := make([]App, len(apps))
	for i, app := range apps {
		iApps[i] = app
	}

	server, err := NewServer(ctx, WithApps(iApps...))
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait for apps to start
	time.Sleep(100 * time.Millisecond)

	// All apps should have been started
	for i, app := range apps {
		if !app.IsRunCalled() {
			t.Errorf("App %d Run method should have been called", i+1)
		}
	}

	// Stop server
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// All apps should have been shut down
	for i, app := range apps {
		if !app.IsShutdownCalled() {
			t.Errorf("App %d Shutdown method should have been called", i+1)
		}
	}
}

// Test_ServerStartHooks tests server startup hooks functionality.
func Test_ServerStartHooks(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer(ctx)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create test hooks
	hook1 := NewTestHook("start-hook-1", nil)
	hook2 := NewTestHook("start-hook-2", nil)

	// Add hooks
	server.AddStartHook(hook1, hook2)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Both hooks should have been executed
	if !hook1.IsExecuted() {
		t.Error("Start hook 1 should have been executed")
	}
	if !hook2.IsExecuted() {
		t.Error("Start hook 2 should have been executed")
	}

	// Stop server
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Stop(stopCtx)
}

// Test_ServerStopHooks tests server shutdown hooks functionality.
func Test_ServerStopHooks(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer(ctx)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create test hooks
	hook1 := NewTestHook("stop-hook-1", nil)
	hook2 := NewTestHook("stop-hook-2", nil)

	// Add hooks
	server.AddStopHook(hook1, hook2)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Stop server
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// Both hooks should have been executed
	if !hook1.IsExecuted() {
		t.Error("Stop hook 1 should have been executed")
	}
	if !hook2.IsExecuted() {
		t.Error("Stop hook 2 should have been executed")
	}
}

// Test_ServerStartHookFailure tests server behavior when a start hook fails.
func Test_ServerStartHookFailure(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer(ctx)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create hooks with one that fails
	hook1 := NewTestHook("good-hook", nil)
	hook2 := NewTestHook("bad-hook", errors.New("hook failed"))

	server.AddStartHook(hook1, hook2)

	// Server start should fail due to hook failure
	if err := server.Start(); err == nil {
		t.Error("Expected server start to fail due to hook failure")
	}

	// Server should not be running
	if server.IsRunning() {
		t.Error("Server should not be running after start hook failure")
	}

	// First hook should have been executed
	if !hook1.IsExecuted() {
		t.Error("First hook should have been executed")
	}

	// Second hook should have been executed (and failed)
	if !hook2.IsExecuted() {
		t.Error("Second hook should have been executed")
	}
}

// Test_ServerAddApp tests dynamic addition of applications.
func Test_ServerAddApp(t *testing.T) {
	ctx := context.Background()
	app1 := NewTestApp("app1", 100*time.Millisecond, nil, nil)

	server, err := NewServer(ctx, WithApp(app1))
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Initially should have 1 app
	if len(server.GetApps()) != 1 {
		t.Errorf("Expected 1 app initially, got %d", len(server.GetApps()))
	}

	// Add more apps dynamically
	app2 := NewTestApp("app2", 100*time.Millisecond, nil, nil)
	app3 := NewTestApp("app3", 100*time.Millisecond, nil, nil)

	server.AddApp(app2, app3)

	// Should now have 3 apps
	if len(server.GetApps()) != 3 {
		t.Errorf("Expected 3 apps after adding, got %d", len(server.GetApps()))
	}
}

// Test_ServerWait tests the Wait method for server completion.
func Test_ServerWait(t *testing.T) {
	ctx := context.Background()
	app := NewTestApp("test-app", 0, nil, nil)

	server, err := NewServer(ctx, WithApp(app))
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Start goroutine to stop server after delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Stop(stopCtx); err != nil {
			t.Errorf("Failed to stop server: %v", err)
		}
	}()

	// Wait for server to stop
	waitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Wait should return nil when server stops normally
	if err := server.Wait(waitCtx); err != nil && err != context.Canceled {
		t.Fatalf("Wait failed: %v", err)
	}

	// Server should no longer be running
	if server.IsRunning() {
		t.Error("Server should not be running after wait completes")
	}
}

// Test_ServerWaitWithContextTimeout tests Wait method with context timeout.
func Test_ServerWaitWithContextTimeout(t *testing.T) {
	ctx := context.Background()
	app := NewTestApp("test-app", 0, nil, nil)

	server, err := NewServer(ctx, WithApp(app))
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait with short timeout (server won't stop in time)
	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = server.Wait(waitCtx)

	// Should get context timeout error
	if err == nil {
		t.Error("Expected timeout error from Wait")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected DeadlineExceeded, got %v", err)
	}

	// Clean up - stop server
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	server.Stop(stopCtx)
}

// Test_ServerWaitWithServerContextCancel tests Wait method when server context is cancelled.
func Test_ServerWaitWithServerContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	app := NewTestApp("test-app", 0, nil, nil)

	server, err := NewServer(ctx, WithApp(app))
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Cancel server context after delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Wait for server
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer waitCancel()

	err = server.Wait(waitCtx)

	// Should get server context cancellation error
	if err == nil {
		t.Error("Expected context cancellation error from Wait")
	}
	if err != context.Canceled {
		t.Errorf("Expected Canceled, got %v", err)
	}
}

// Test_ServerIsRunning tests the IsRunning method.
func Test_ServerIsRunning(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer(ctx)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Should not be running initially
	if server.IsRunning() {
		t.Error("Server should not be running initially")
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Should be running after start
	if !server.IsRunning() {
		t.Error("Server should be running after start")
	}

	// Stop server
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// Should not be running after stop
	if server.IsRunning() {
		t.Error("Server should not be running after stop")
	}
}

// Test_ServerDoubleStart tests that starting an already running server is safe.
func Test_ServerDoubleStart(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer(ctx)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server first time
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server first time: %v", err)
	}

	// Start server second time (should be safe)
	if err := server.Start(); err != nil {
		t.Errorf("Second start should be safe: %v", err)
	}

	// Should still be running
	if !server.IsRunning() {
		t.Error("Server should still be running")
	}

	// Clean up
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Stop(stopCtx)
}

// Test_ServerDoubleStop tests that stopping an already stopped server is safe.
func Test_ServerDoubleStop(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer(ctx)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop server first time
	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop server first time: %v", err)
	}

	// Stop server second time (should be safe)
	if err := server.Stop(stopCtx); err != nil {
		t.Errorf("Second stop should be safe: %v", err)
	}

	// Should not be running
	if server.IsRunning() {
		t.Error("Server should not be running")
	}
}

// Test_HookFunc tests the HookFunc type implementation.
func Test_HookFunc(t *testing.T) {
	executed := false

	// Create a HookFunc
	hook := HookFunc(func(ctx context.Context) error {
		executed = true
		return nil
	})

	// Execute the hook
	ctx := context.Background()
	if err := hook.Run(ctx); err != nil {
		t.Errorf("Hook execution failed: %v", err)
	}

	// Should have been executed
	if !executed {
		t.Error("Hook should have been executed")
	}
}

// Test_HookFuncWithError tests HookFunc error handling.
func Test_HookFuncWithError(t *testing.T) {
	expectedErr := errors.New("hook error")

	// Create a HookFunc that returns an error
	hook := HookFunc(func(ctx context.Context) error {
		return expectedErr
	})

	// Execute the hook
	ctx := context.Background()
	err := hook.Run(ctx)

	// Should return the expected error
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}

// Test_ServerWithAppErrors tests server behavior when applications return errors.
func Test_ServerWithAppErrors(t *testing.T) {
	ctx := context.Background()

	// Create apps with different error conditions
	goodApp := NewTestApp("good-app", 0, nil, nil)
	runErrorApp := NewTestApp("run-error-app", 100*time.Millisecond, errors.New("run error"), nil)
	shutdownErrorApp := NewTestApp("shutdown-error-app", 0, nil, errors.New("shutdown error"))

	server, err := NewServer(ctx, WithApps(goodApp, runErrorApp, shutdownErrorApp))
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server (should succeed despite app run errors)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait for run error app to complete
	time.Sleep(200 * time.Millisecond)

	// Stop server (should succeed despite app shutdown errors)
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// All apps should have been called appropriately
	if !goodApp.IsRunCalled() {
		t.Error("Good app Run should have been called")
	}
	if !goodApp.IsShutdownCalled() {
		t.Error("Good app Shutdown should have been called")
	}
	if !runErrorApp.IsRunCalled() {
		t.Error("Run error app Run should have been called")
	}
	if !shutdownErrorApp.IsRunCalled() {
		t.Error("Shutdown error app Run should have been called")
	}
	if !shutdownErrorApp.IsShutdownCalled() {
		t.Error("Shutdown error app Shutdown should have been called")
	}
}

// Test_ServerStopHookErrors tests server behavior when stop hooks return errors.
func Test_ServerStopHookErrors(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer(ctx)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create hooks with different error conditions
	goodHook1 := NewTestHook("good-hook-1", nil)
	errorHook := NewTestHook("error-hook", errors.New("hook error"))
	goodHook2 := NewTestHook("good-hook-2", nil)

	server.AddStopHook(goodHook1, errorHook, goodHook2)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Stop server (should succeed despite hook errors)
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		t.Fatalf("Server stop should succeed despite hook errors: %v", err)
	}

	// All hooks should have been executed
	if !goodHook1.IsExecuted() {
		t.Error("Good hook 1 should have been executed")
	}
	if !errorHook.IsExecuted() {
		t.Error("Error hook should have been executed")
	}
	if !goodHook2.IsExecuted() {
		t.Error("Good hook 2 should have been executed")
	}
}

// Benchmark_ServerWait benchmarks the Wait method performance.
func Benchmark_ServerWait(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		server, err := NewServer(ctx)
		if err != nil {
			b.Fatalf("Failed to create server: %v", err)
		}

		// Start server
		if err := server.Start(); err != nil {
			b.Fatalf("Failed to start server: %v", err)
		}

		// Stop server immediately to make it ready for waiting
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Stop(stopCtx); err != nil {
			b.Fatalf("Failed to stop server: %v", err)
		}

		// Wait should return immediately since server is stopped
		waitCtx, waitCancel := context.WithTimeout(context.Background(), 1*time.Second)
		err = server.Wait(waitCtx)
		waitCancel()

		// Wait can return nil (stopped normally) or context.Canceled (server context cancelled)
		if err != nil && err != context.Canceled {
			b.Fatalf("Wait failed: %v", err)
		}
	}
}

// Example_basicUsage demonstrates basic server usage.
func Example_basicUsage() {
	// Create a context
	ctx := context.Background()

	// Create test applications
	app1 := NewTestApp("web-server", 0, nil, nil)
	app2 := NewTestApp("background-worker", 0, nil, nil)

	// Create server with applications
	server, err := NewServer(ctx, WithApps(app1, app2))
	if err != nil {
		log.Fatal(err)
	}

	// Add startup hook
	server.AddStartHook(HookFunc(func(ctx context.Context) error {
		fmt.Println("Initializing resources...")
		return nil
	}))

	// Add shutdown hook
	server.AddStopHook(HookFunc(func(ctx context.Context) error {
		fmt.Println("Cleaning up resources...")
		return nil
	}))

	// Start server
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}

	// In a real application, you would wait for server to complete
	// For this example, we'll stop it immediately
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(stopCtx); err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	fmt.Println("Server stopped")
	// Output: Server stopped
}

// Example_withWait demonstrates using the Wait method.
func Example_withWait() {
	ctx := context.Background()
	app := NewTestApp("example-app", 2*time.Second, nil, nil)

	server, err := NewServer(ctx, WithApp(app))
	if err != nil {
		log.Fatal(err)
	}

	// Start server
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}

	// Start a goroutine to stop server after some time
	go func() {
		time.Sleep(1 * time.Second)
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(stopCtx)
	}()

	// Wait for server to complete
	waitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Wait(waitCtx); err != nil {
		log.Printf("Wait error: %v", err)
	}

	fmt.Println("Server completed")
	// Output: Server completed
}
