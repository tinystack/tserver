# TServer

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.18-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Test Coverage](https://img.shields.io/badge/coverage-98.1%25-brightgreen.svg)](https://github.com/tinystack/tserver)

A robust, lightweight server framework for Go that simplifies managing multiple applications with lifecycle hooks, graceful shutdown, and signal handling capabilities.

## Features

- **Multi-Application Management**: Run multiple applications within a single server instance
- **Lifecycle Hooks**: Add startup and shutdown hooks for initialization and cleanup tasks
- **Graceful Shutdown**: Automatic handling of SIGINT and SIGTERM signals with configurable timeouts
- **Context-Based Control**: Full context support for cancellation and timeout management
- **Thread-Safe Operations**: Concurrent-safe application and hook management
- **Flexible Architecture**: Easy to extend and customize for specific needs
- **Comprehensive Testing**: 98.1% test coverage with extensive unit tests
- **Zero Dependencies**: No external dependencies beyond Go standard library

## Installation

```bash
go get github.com/tinystack/tserver
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/tinystack/tserver"
)

// Implement your application
type MyApp struct{}

func (a *MyApp) Run(ctx context.Context) error {
    // Your application logic here
    <-ctx.Done() // Wait for cancellation
    return ctx.Err()
}

func (a *MyApp) Shutdown(ctx context.Context) error {
    // Cleanup logic here
    log.Println("App shutting down...")
    return nil
}

func main() {
    ctx := context.Background()
    app := &MyApp{}

    // Create server with application
    server, err := tserver.NewServer(ctx, tserver.WithApp(app))
    if err != nil {
        log.Fatal(err)
    }

    // Start the server
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }

    // Wait for graceful shutdown
    if err := server.Wait(ctx); err != nil {
        log.Printf("Server wait error: %v", err)
    }

    log.Println("Server stopped gracefully")
}
```

### Advanced Usage with Multiple Applications

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/tinystack/tserver"
)

func main() {
    ctx := context.Background()

    // Create multiple applications
    webApp := &WebServerApp{addr: ":8080"}
    worker := &BackgroundWorker{interval: time.Second * 5}
    scheduler := &TaskScheduler{}

    // Create server with multiple applications
    server, err := tserver.NewServer(ctx, tserver.WithApps(
        webApp, worker, scheduler,
    ))
    if err != nil {
        log.Fatal(err)
    }

    // Add startup hooks
    server.AddStartHook(tserver.HookFunc(func(ctx context.Context) error {
        log.Println("Loading configuration...")
        return loadConfig()
    }))

    server.AddStartHook(tserver.HookFunc(func(ctx context.Context) error {
        log.Println("Initializing database...")
        return initDatabase()
    }))

    // Add shutdown hooks
    server.AddStopHook(tserver.HookFunc(func(ctx context.Context) error {
        log.Println("Saving application state...")
        return saveState()
    }))

    // Start server
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }

    log.Println("Server started successfully!")
    log.Println("Press Ctrl+C to stop...")

    // Wait for shutdown
    if err := server.Wait(ctx); err != nil {
        log.Printf("Server error: %v", err)
    }

    log.Println("All applications stopped gracefully")
}
```

## Core Interfaces

### App Interface

All applications must implement the `App` interface:

```go
type App interface {
    // Run starts the application and blocks until context is cancelled
    Run(ctx context.Context) error

    // Shutdown performs graceful shutdown within the timeout
    Shutdown(ctx context.Context) error
}
```

### Hook Interface

Lifecycle hooks implement the `Hook` interface:

```go
type Hook interface {
    // Run executes the hook logic
    Run(ctx context.Context) error
}

// HookFunc allows regular functions to be used as hooks
type HookFunc func(ctx context.Context) error

func (h HookFunc) Run(ctx context.Context) error {
    return h(ctx)
}
```

## API Reference

### Server Creation

#### `NewServer(ctx context.Context, options ...Option) (*Server, error)`

Creates a new server instance with the given context and options.

**Options:**

- `WithApp(app App)` - Add a single application
- `WithApps(apps ...App)` - Add multiple applications

```go
// Single application
server, err := tserver.NewServer(ctx, tserver.WithApp(myApp))

// Multiple applications
server, err := tserver.NewServer(ctx, tserver.WithApps(app1, app2, app3))
```

### Server Management

#### `Start() error`

Starts the server and all registered applications. This method:

1. Executes all startup hooks sequentially
2. Starts signal handling
3. Launches all applications concurrently
4. Marks server as running

```go
if err := server.Start(); err != nil {
    log.Fatal("Failed to start server:", err)
}
```

#### `Stop(ctx context.Context) error`

Initiates graceful shutdown of the server. This method:

1. Shuts down all applications concurrently
2. Executes all shutdown hooks sequentially
3. Cancels server context
4. Closes done channel

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := server.Stop(ctx); err != nil {
    log.Printf("Stop error: %v", err)
}
```

#### `Wait(ctx context.Context) error`

Blocks until the server stops or context is cancelled.

```go
// Wait indefinitely
if err := server.Wait(context.Background()); err != nil {
    log.Printf("Wait error: %v", err)
}

// Wait with timeout
ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
defer cancel()

if err := server.Wait(ctx); err != nil {
    if err == context.DeadlineExceeded {
        log.Println("Server didn't stop within timeout")
    } else {
        log.Printf("Wait error: %v", err)
    }
}
```

### Dynamic Management

#### `AddApp(apps ...App)`

Dynamically add applications to the server (thread-safe).

```go
server.AddApp(newApp1, newApp2)
```

#### `GetApps() []App`

Get a copy of all managed applications (thread-safe).

```go
apps := server.GetApps()
fmt.Printf("Managing %d applications\n", len(apps))
```

#### `IsRunning() bool`

Check if the server is currently running (thread-safe).

```go
if server.IsRunning() {
    log.Println("Server is running")
}
```

### Lifecycle Hooks

#### `AddStartHook(hooks ...Hook)`

Add hooks to be executed during server startup.

```go
server.AddStartHook(
    tserver.HookFunc(func(ctx context.Context) error {
        return initializeResources()
    }),
    tserver.HookFunc(func(ctx context.Context) error {
        return loadConfiguration()
    }),
)
```

#### `AddStopHook(hooks ...Hook)`

Add hooks to be executed during server shutdown.

```go
server.AddStopHook(
    tserver.HookFunc(func(ctx context.Context) error {
        return saveApplicationState()
    }),
    tserver.HookFunc(func(ctx context.Context) error {
        return cleanupResources()
    }),
)
```

### Signal Handling

#### `AddStopSignal(signals ...os.Signal)`

Add custom signals that trigger graceful shutdown.

```go
server.AddStopSignal(syscall.SIGUSR1, syscall.SIGUSR2)
```

**Default signals:** `SIGINT`, `SIGTERM`

## Examples

### HTTP Server Application

```go
type WebServerApp struct {
    server *http.Server
    addr   string
}

func NewWebServerApp(addr string) *WebServerApp {
    return &WebServerApp{addr: addr}
}

func (w *WebServerApp) Run(ctx context.Context) error {
    w.server = &http.Server{
        Addr:    w.addr,
        Handler: http.DefaultServeMux,
    }

    errChan := make(chan error, 1)
    go func() {
        if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            errChan <- err
        }
    }()

    select {
    case <-ctx.Done():
        return ctx.Err()
    case err := <-errChan:
        return err
    }
}

func (w *WebServerApp) Shutdown(ctx context.Context) error {
    if w.server != nil {
        return w.server.Shutdown(ctx)
    }
    return nil
}
```

### Background Worker Application

```go
type BackgroundWorker struct {
    name     string
    interval time.Duration
}

func (w *BackgroundWorker) Run(ctx context.Context) error {
    ticker := time.NewTicker(w.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := w.doWork(ctx); err != nil {
                log.Printf("Work error: %v", err)
            }
        }
    }
}

func (w *BackgroundWorker) Shutdown(ctx context.Context) error {
    log.Printf("Worker %s shutting down...", w.name)
    return nil
}

func (w *BackgroundWorker) doWork(ctx context.Context) error {
    // Simulate work
    log.Printf("Worker %s performing task...", w.name)
    time.Sleep(100 * time.Millisecond)
    return nil
}
```

## Best Practices

### 1. Context Handling

Always respect context cancellation in your applications:

```go
func (a *MyApp) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err() // Respect cancellation
        default:
            // Do work
        }
    }
}
```

### 2. Error Handling

Handle errors gracefully in applications and hooks:

```go
func (a *MyApp) Run(ctx context.Context) error {
    if err := a.initialize(); err != nil {
        return fmt.Errorf("initialization failed: %w", err)
    }

    // Main application loop
    return a.mainLoop(ctx)
}
```

### 3. Resource Cleanup

Always clean up resources in the Shutdown method:

```go
func (a *MyApp) Shutdown(ctx context.Context) error {
    // Close connections, files, etc.
    if a.db != nil {
        return a.db.Close()
    }
    return nil
}
```

### 4. Timeout Management

Use appropriate timeouts for shutdown operations:

```go
func main() {
    // ... server setup ...

    if err := server.Start(); err != nil {
        log.Fatal(err)
    }

    // Wait with reasonable timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Wait(ctx); err != nil {
        log.Printf("Shutdown timeout: %v", err)
    }
}
```

## Error Handling

TServer handles errors gracefully:

- **Start Hook Failures**: If any start hook fails, server startup is aborted
- **Application Errors**: Application runtime errors are logged but don't stop other applications
- **Stop Hook Failures**: Stop hook errors are logged but don't prevent shutdown completion
- **Shutdown Errors**: Application shutdown errors are logged but don't interrupt the shutdown process

## Thread Safety

All server operations are thread-safe:

- `AddApp()`, `GetApps()`, `IsRunning()` can be called concurrently
- Hook management is protected by mutexes
- Server state changes are atomic

## Testing

The package includes comprehensive tests with 98.1% coverage:

```bash
go test -v
go test -cover
go test -bench=.
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -am 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Changelog

### v1.0.0

- Initial release
- Multi-application management
- Lifecycle hooks support
- Graceful shutdown
- Signal handling
- Context-based control
- Comprehensive test suite

---

For more examples and detailed documentation, please check the [example](example/) directory.
