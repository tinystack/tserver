// Package tserver provides a robust server framework for managing multiple applications
// with lifecycle hooks, graceful shutdown, and signal handling capabilities.
//
// The tserver package allows you to:
//   - Manage multiple applications within a single server instance
//   - Add startup and shutdown hooks for initialization and cleanup
//   - Handle system signals for graceful shutdown
//   - Wait for server completion with context-based timeout control
//
// Example usage:
//
//	server, err := tserver.NewServer(ctx, tserver.WithApps(app1, app2))
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	server.AddStartHook(tserver.HookFunc(func(ctx context.Context) error {
//		log.Println("Initializing resources...")
//		return nil
//	}))
//
//	if err := server.Start(); err != nil {
//		log.Fatal(err)
//	}
//
//	// Wait for server to stop gracefully
//	if err := server.Wait(ctx); err != nil {
//		log.Printf("Server wait error: %v", err)
//	}
package tserver

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// App defines the interface that applications must implement to be managed by the server.
// Applications are responsible for their own lifecycle management and must respond
// to context cancellation for graceful shutdown.
type App interface {
	// Run starts the application and blocks until the context is cancelled
	// or an error occurs. It should return when ctx.Done() is closed.
	Run(ctx context.Context) error

	// Shutdown performs graceful shutdown operations. It should clean up
	// resources and stop any background processes within the given context timeout.
	Shutdown(ctx context.Context) error
}

// Hook defines the interface for lifecycle hooks that can be executed
// during server startup and shutdown phases.
type Hook interface {
	// Run executes the hook logic. It should complete within a reasonable time
	// and respect the provided context for cancellation.
	Run(ctx context.Context) error
}

// HookFunc is a function type that implements the Hook interface.
// It allows regular functions to be used as hooks without implementing
// the Hook interface explicitly.
type HookFunc func(ctx context.Context) error

// Run implements the Hook interface for HookFunc.
func (h HookFunc) Run(ctx context.Context) error {
	return h(ctx)
}

// Option defines a function type for configuring Server instances.
// Options are applied during server creation to customize behavior.
type Option func(*Server)

// WithApp adds a single application to the server configuration.
// Multiple WithApp calls can be chained to add multiple applications.
//
// Example:
//
//	server := NewServer(ctx, WithApp(app1), WithApp(app2))
func WithApp(app App) Option {
	return func(s *Server) {
		s.apps = append(s.apps, app)
	}
}

// WithApps adds multiple applications to the server configuration in a single call.
// This is more efficient than multiple WithApp calls when adding many applications.
//
// Example:
//
//	server := NewServer(ctx, WithApps(app1, app2, app3))
func WithApps(apps ...App) Option {
	return func(s *Server) {
		s.apps = append(s.apps, apps...)
	}
}

// Server manages multiple applications and their lifecycle.
// It provides hooks for startup and shutdown, signal handling,
// and graceful shutdown capabilities.
type Server struct {
	// Context and cancellation
	ctx    context.Context    // Server context for cancellation propagation
	cancel context.CancelFunc // Function to cancel the server context

	// Applications and hooks
	apps       []App  // List of applications managed by this server
	startHooks []Hook // Hooks executed before starting applications
	stopHooks  []Hook // Hooks executed after stopping applications

	// Signal handling
	stopSignals []os.Signal // OS signals that trigger graceful shutdown

	// State management
	running bool          // Current running state of the server
	done    chan struct{} // Channel closed when server stops
	mu      sync.RWMutex  // Mutex for thread-safe state access
}

// NewServer creates a new server instance with the provided context and options.
// The context is used for cancellation propagation to all managed applications.
// Default stop signals (SIGINT, SIGTERM) are automatically configured.
//
// Parameters:
//   - ctx: Base context for the server and all applications
//   - opts: Configuration options to customize server behavior
//
// Returns:
//   - *Server: Configured server instance
//   - error: Configuration error, if any
func NewServer(ctx context.Context, opts ...Option) (*Server, error) {
	ctx, cancel := context.WithCancel(ctx)

	server := &Server{
		ctx:         ctx,
		cancel:      cancel,
		apps:        make([]App, 0),
		startHooks:  make([]Hook, 0),
		stopHooks:   make([]Hook, 0),
		stopSignals: []os.Signal{syscall.SIGINT, syscall.SIGTERM},
		running:     false,
		done:        make(chan struct{}),
	}

	// Apply configuration options
	for _, opt := range opts {
		opt(server)
	}

	logInfo("Server created successfully")
	return server, nil
}

// Start begins the server lifecycle by executing startup hooks and launching applications.
// This method is idempotent - calling it multiple times has no additional effect.
//
// The startup process:
//  1. Execute all startup hooks in sequence
//  2. Start signal handling in a separate goroutine
//  3. Launch all applications concurrently
//  4. Mark server as running
//
// Returns:
//   - error: Startup error from hooks or configuration issues
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		logWarn("Server is already running")
		return nil
	}

	logInfo("Starting server...")

	// Execute startup hooks sequentially
	for i, hook := range s.startHooks {
		logInfo(fmt.Sprintf("Executing start hook %d/%d", i+1, len(s.startHooks)))
		if err := hook.Run(s.ctx); err != nil {
			logError(fmt.Sprintf("Start hook %d failed: %v", i+1, err))
			return fmt.Errorf("start hook %d failed: %w", i+1, err)
		}
	}

	// Start signal handling in background
	go s.handleSignals()

	// Launch all applications concurrently
	if len(s.apps) > 0 {
		logInfo(fmt.Sprintf("Starting %d application(s)...", len(s.apps)))
		for i, app := range s.apps {
			appIndex := i + 1
			go func(appIdx int, application App) {
				logInfo(fmt.Sprintf("Starting application %d/%d...", appIdx, len(s.apps)))
				if err := application.Run(s.ctx); err != nil {
					logError(fmt.Sprintf("Application %d error: %v", appIdx, err))
				}
			}(appIndex, app)
		}
	}

	s.running = true
	logInfo("Server started successfully")
	return nil
}

// Stop initiates graceful shutdown of the server and all managed applications.
// This method can be called multiple times safely.
//
// The shutdown process:
//  1. Shutdown all applications concurrently
//  2. Execute all shutdown hooks in sequence
//  3. Cancel the server context
//  4. Close the done channel to notify waiting goroutines
//
// Parameters:
//   - ctx: Context for shutdown timeout control
//
// Returns:
//   - error: Always nil (shutdown errors are logged but don't prevent completion)
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		logWarn("Server is not running")
		return nil
	}

	logInfo("Stopping server...")

	// Shutdown all applications concurrently
	if len(s.apps) > 0 {
		logInfo(fmt.Sprintf("Shutting down %d application(s)...", len(s.apps)))
		var wg sync.WaitGroup
		for i, app := range s.apps {
			wg.Add(1)
			appIndex := i + 1
			go func(appIdx int, application App) {
				defer wg.Done()
				logInfo(fmt.Sprintf("Shutting down application %d/%d...", appIdx, len(s.apps)))
				if err := application.Shutdown(ctx); err != nil {
					logError(fmt.Sprintf("Application %d shutdown error: %v", appIdx, err))
					// Continue with other applications - don't interrupt shutdown process
				}
			}(appIndex, app)
		}
		wg.Wait()
	}

	// Execute shutdown hooks sequentially
	for i, hook := range s.stopHooks {
		logInfo(fmt.Sprintf("Executing stop hook %d/%d", i+1, len(s.stopHooks)))
		if err := hook.Run(ctx); err != nil {
			logError(fmt.Sprintf("Stop hook %d failed: %v", i+1, err))
			// Continue with other hooks - don't interrupt shutdown process
		}
	}

	// Cancel server context and notify waiters
	s.cancel()
	s.running = false

	// Notify waiting goroutines that server has stopped
	close(s.done)

	logInfo("Server stopped successfully")
	return nil
}

// Wait blocks until the server stops or the provided context is cancelled.
// This method allows graceful waiting for server completion with timeout control.
//
// The method returns when:
//   - Server stops normally (returns nil)
//   - Provided context is cancelled (returns ctx.Err())
//   - Server context is cancelled (returns server context error)
//
// Parameters:
//   - ctx: Context for timeout and cancellation control
//
// Returns:
//   - error: nil if server stopped normally, context error otherwise
func (s *Server) Wait(ctx context.Context) error {
	select {
	case <-s.done:
		// Server has stopped normally
		return nil
	case <-ctx.Done():
		// Provided context was cancelled (timeout or manual cancellation)
		return ctx.Err()
	case <-s.ctx.Done():
		// Server context was cancelled
		return s.ctx.Err()
	}
}

// AddStartHook adds one or more hooks to be executed during server startup.
// Start hooks are executed sequentially before applications are launched.
// If any start hook fails, the server startup process is aborted.
//
// Parameters:
//   - hooks: One or more hooks to add to the startup sequence
func (s *Server) AddStartHook(hooks ...Hook) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.startHooks = append(s.startHooks, hooks...)
	logInfo(fmt.Sprintf("Added %d start hook(s), total: %d", len(hooks), len(s.startHooks)))
}

// AddStopHook adds one or more hooks to be executed during server shutdown.
// Stop hooks are executed sequentially after applications are shut down.
// Stop hook failures are logged but don't prevent shutdown completion.
//
// Parameters:
//   - hooks: One or more hooks to add to the shutdown sequence
func (s *Server) AddStopHook(hooks ...Hook) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopHooks = append(s.stopHooks, hooks...)
	logInfo(fmt.Sprintf("Added %d stop hook(s), total: %d", len(hooks), len(s.stopHooks)))
}

// AddStopSignal adds one or more OS signals that will trigger graceful shutdown.
// Default signals (SIGINT, SIGTERM) are automatically configured.
//
// Parameters:
//   - signals: One or more OS signals to monitor for shutdown
func (s *Server) AddStopSignal(signals ...os.Signal) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stopSignals = append(s.stopSignals, signals...)
	logInfo(fmt.Sprintf("Added %d stop signal(s), total: %d", len(signals), len(s.stopSignals)))
}

// AddApp dynamically adds one or more applications to the server.
// Applications added after server startup will not be started automatically.
// This method is thread-safe and can be called concurrently.
//
// Parameters:
//   - apps: One or more applications to add to the server
func (s *Server) AddApp(apps ...App) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.apps = append(s.apps, apps...)
	logInfo(fmt.Sprintf("Added %d application(s), total: %d", len(apps), len(s.apps)))
}

// GetApps returns a copy of all applications managed by the server.
// The returned slice is a copy to prevent external modification of the server state.
// This method is thread-safe.
//
// Returns:
//   - []App: Copy of all managed applications
func (s *Server) GetApps() []App {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	apps := make([]App, len(s.apps))
	copy(apps, s.apps)
	return apps
}

// IsRunning returns the current running state of the server.
// This method is thread-safe and can be called concurrently.
//
// Returns:
//   - bool: true if server is currently running, false otherwise
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// handleSignals monitors configured OS signals and triggers graceful shutdown.
// This method runs in a separate goroutine and handles signal processing.
// When a signal is received, it initiates shutdown with a 30-second timeout.
func (s *Server) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, s.stopSignals...)

	select {
	case sig := <-sigChan:
		logInfo(fmt.Sprintf("Received signal: %v", sig))
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.Stop(ctx); err != nil {
			logError(fmt.Sprintf("Failed to stop server gracefully: %v", err))
		}
	case <-s.ctx.Done():
		// Server context cancelled, stop signal handling
		return
	}
}

// Logging functions for server events

// logInfo logs informational messages with timestamp and component prefix
func logInfo(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [TSERVER] [INFO] %s\n", timestamp, message)
}

// logError logs error messages with timestamp and component prefix
func logError(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [TSERVER] [ERROR] %s\n", timestamp, message)
}

// logWarn logs warning messages with timestamp and component prefix
func logWarn(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [TSERVER] [WARN] %s\n", timestamp, message)
}
