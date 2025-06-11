package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/tinystack/tserver"
)

// WebServerApp demonstrates a simple HTTP server application
// that implements the tserver.App interface.
type WebServerApp struct {
	server *http.Server
	addr   string
}

// NewWebServerApp creates a new web server application instance.
//
// Parameters:
//   - addr: The address to bind the HTTP server to (e.g., ":8080")
//
// Returns:
//   - *WebServerApp: Configured web server application
func NewWebServerApp(addr string) *WebServerApp {
	return &WebServerApp{
		addr: addr,
	}
}

// Run starts the HTTP server and blocks until the context is cancelled.
// This method implements the tserver.App interface.
func (w *WebServerApp) Run(ctx context.Context) error {
	// Create HTTP server with simple handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(rw, "Hello from Web Server! Time: %s", time.Now().Format(time.RFC3339))
	})
	mux.HandleFunc("/health", func(rw http.ResponseWriter, r *http.Request) {
		fmt.Fprint(rw, "OK")
	})

	w.server = &http.Server{
		Addr:    w.addr,
		Handler: mux,
	}

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		log.Printf("[WEB] Starting HTTP server on %s", w.addr)
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		log.Println("[WEB] Context cancelled, stopping HTTP server...")
		return ctx.Err()
	case err := <-errChan:
		log.Printf("[WEB] HTTP server error: %v", err)
		return err
	}
}

// Shutdown performs graceful shutdown of the HTTP server.
// This method implements the tserver.App interface.
func (w *WebServerApp) Shutdown(ctx context.Context) error {
	if w.server == nil {
		return nil
	}

	log.Println("[WEB] Shutting down HTTP server...")
	return w.server.Shutdown(ctx)
}

// WorkerApp demonstrates a background worker application
// that implements the tserver.App interface.
type WorkerApp struct {
	name     string
	interval time.Duration
}

// NewWorkerApp creates a new worker application instance.
//
// Parameters:
//   - name: Human-readable name for the worker
//   - interval: Time interval between work iterations
//
// Returns:
//   - *WorkerApp: Configured worker application
func NewWorkerApp(name string, interval time.Duration) *WorkerApp {
	return &WorkerApp{
		name:     name,
		interval: interval,
	}
}

// Run starts the worker and performs periodic work until context is cancelled.
// This method implements the tserver.App interface.
func (w *WorkerApp) Run(ctx context.Context) error {
	log.Printf("[%s] Starting background worker (interval: %s)", w.name, w.interval)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[%s] Context cancelled, stopping worker...", w.name)
			return ctx.Err()
		case <-ticker.C:
			// Simulate some work
			workDuration := time.Duration(rand.Intn(100)) * time.Millisecond
			log.Printf("[%s] Performing work (duration: %s)...", w.name, workDuration)
			time.Sleep(workDuration)
			log.Printf("[%s] Work completed", w.name)
		}
	}
}

// Shutdown performs graceful shutdown of the worker.
// This method implements the tserver.App interface.
func (w *WorkerApp) Shutdown(ctx context.Context) error {
	log.Printf("[%s] Worker shutdown completed", w.name)
	return nil
}

// DatabaseApp demonstrates a database connection application
// that implements the tserver.App interface.
type DatabaseApp struct {
	connectionString string
	connected        bool
}

// NewDatabaseApp creates a new database application instance.
//
// Parameters:
//   - connectionString: Database connection string (simulated)
//
// Returns:
//   - *DatabaseApp: Configured database application
func NewDatabaseApp(connectionString string) *DatabaseApp {
	return &DatabaseApp{
		connectionString: connectionString,
		connected:        false,
	}
}

// Run starts the database connection and maintains it until context is cancelled.
// This method implements the tserver.App interface.
func (d *DatabaseApp) Run(ctx context.Context) error {
	log.Printf("[DB] Connecting to database: %s", d.connectionString)

	// Simulate connection establishment
	time.Sleep(500 * time.Millisecond)
	d.connected = true
	log.Println("[DB] Database connection established")

	// Keep connection alive until context is cancelled
	<-ctx.Done()
	log.Println("[DB] Context cancelled, closing database connection...")
	return ctx.Err()
}

// Shutdown performs graceful shutdown of the database connection.
// This method implements the tserver.App interface.
func (d *DatabaseApp) Shutdown(ctx context.Context) error {
	if !d.connected {
		return nil
	}

	log.Println("[DB] Closing database connection...")
	// Simulate connection cleanup
	time.Sleep(200 * time.Millisecond)
	d.connected = false
	log.Println("[DB] Database connection closed")
	return nil
}

// SchedulerApp demonstrates a task scheduler application
// that implements the tserver.App interface.
type SchedulerApp struct {
	tasks []string
}

// NewSchedulerApp creates a new scheduler application instance.
//
// Parameters:
//   - tasks: List of task names to schedule
//
// Returns:
//   - *SchedulerApp: Configured scheduler application
func NewSchedulerApp(tasks []string) *SchedulerApp {
	return &SchedulerApp{
		tasks: tasks,
	}
}

// Run starts the task scheduler and processes tasks until context is cancelled.
// This method implements the tserver.App interface.
func (s *SchedulerApp) Run(ctx context.Context) error {
	log.Printf("[SCHEDULER] Starting task scheduler with %d tasks", len(s.tasks))

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	taskIndex := 0
	for {
		select {
		case <-ctx.Done():
			log.Println("[SCHEDULER] Context cancelled, stopping scheduler...")
			return ctx.Err()
		case <-ticker.C:
			if len(s.tasks) > 0 {
				task := s.tasks[taskIndex%len(s.tasks)]
				log.Printf("[SCHEDULER] Executing task: %s", task)

				// Simulate task execution
				time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
				log.Printf("[SCHEDULER] Task completed: %s", task)

				taskIndex++
			}
		}
	}
}

// Shutdown performs graceful shutdown of the scheduler.
// This method implements the tserver.App interface.
func (s *SchedulerApp) Shutdown(ctx context.Context) error {
	log.Println("[SCHEDULER] Scheduler shutdown completed")
	return nil
}

func main() {
	// Create root context for the application
	ctx := context.Background()

	// Create various application instances
	webApp := NewWebServerApp(":8080")
	worker1 := NewWorkerApp("WORKER-1", 2*time.Second)
	worker2 := NewWorkerApp("WORKER-2", 5*time.Second)
	dbApp := NewDatabaseApp("postgresql://localhost:5432/mydb")
	schedulerApp := NewSchedulerApp([]string{
		"cleanup-temp-files",
		"send-email-notifications",
		"update-statistics",
		"backup-database",
	})

	// Create server with all applications
	server, err := tserver.NewServer(ctx, tserver.WithApps(
		webApp,
		worker1,
		worker2,
		dbApp,
		schedulerApp,
	))
	if err != nil {
		log.Fatal("Failed to create server:", err)
	}

	// Add startup hooks for initialization tasks
	server.AddStartHook(tserver.HookFunc(func(ctx context.Context) error {
		log.Println("[STARTUP] Initializing application resources...")
		time.Sleep(100 * time.Millisecond) // Simulate initialization
		log.Println("[STARTUP] Application resources initialized")
		return nil
	}))

	server.AddStartHook(tserver.HookFunc(func(ctx context.Context) error {
		log.Println("[STARTUP] Loading configuration...")
		time.Sleep(50 * time.Millisecond) // Simulate config loading
		log.Println("[STARTUP] Configuration loaded successfully")
		return nil
	}))

	// Add shutdown hooks for cleanup tasks
	server.AddStopHook(tserver.HookFunc(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Saving application state...")
		time.Sleep(100 * time.Millisecond) // Simulate state saving
		log.Println("[SHUTDOWN] Application state saved")
		return nil
	}))

	server.AddStopHook(tserver.HookFunc(func(ctx context.Context) error {
		log.Println("[SHUTDOWN] Cleaning up temporary resources...")
		time.Sleep(50 * time.Millisecond) // Simulate cleanup
		log.Println("[SHUTDOWN] Temporary resources cleaned up")
		return nil
	}))

	// Start the server
	log.Println("Starting tserver with multiple applications...")
	if err := server.Start(); err != nil {
		log.Fatal("Failed to start server:", err)
	}

	log.Println("Server started successfully!")
	log.Println("Try visiting: http://localhost:8080")
	log.Println("Health check: http://localhost:8080/health")
	log.Println("Press Ctrl+C to gracefully stop the server")

	// Wait for server to complete (will block until signal is received)
	waitCtx := context.Background()
	if err := server.Wait(waitCtx); err != nil {
		log.Printf("Server wait error: %v", err)
	}

	log.Println("Server stopped gracefully")
}
