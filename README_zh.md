# TServer

[![Go 版本](https://img.shields.io/badge/go-%3E%3D1.18-blue.svg)](https://golang.org/)
[![许可证](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![测试覆盖率](https://img.shields.io/badge/coverage-98.1%25-brightgreen.svg)](https://github.com/tinystack/tserver)

一个强大、轻量级的 Go 服务器框架，简化多应用程序管理，支持生命周期钩子、优雅关闭和信号处理功能。

## 特性

- **多应用程序管理**：在单个服务器实例中运行多个应用程序
- **生命周期钩子**：添加启动和关闭钩子用于初始化和清理任务
- **优雅关闭**：自动处理 SIGINT 和 SIGTERM 信号，支持可配置超时
- **基于上下文控制**：完全支持上下文的取消和超时管理
- **线程安全操作**：并发安全的应用程序和钩子管理
- **灵活架构**：易于扩展和自定义特定需求
- **全面测试**：98.1% 测试覆盖率，包含大量单元测试
- **零依赖**：除 Go 标准库外无外部依赖

## 安装

```bash
go get github.com/tinystack/tserver
```

## 快速开始

### 基本用法

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/tinystack/tserver"
)

// 实现您的应用程序
type MyApp struct{}

func (a *MyApp) Run(ctx context.Context) error {
    // 您的应用程序逻辑
    <-ctx.Done() // 等待取消
    return ctx.Err()
}

func (a *MyApp) Shutdown(ctx context.Context) error {
    // 清理逻辑
    log.Println("应用程序正在关闭...")
    return nil
}

func main() {
    ctx := context.Background()
    app := &MyApp{}

    // 创建带应用程序的服务器
    server, err := tserver.NewServer(ctx, tserver.WithApp(app))
    if err != nil {
        log.Fatal(err)
    }

    // 启动服务器
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }

    // 等待优雅关闭
    if err := server.Wait(ctx); err != nil {
        log.Printf("服务器等待错误: %v", err)
    }

    log.Println("服务器已优雅停止")
}
```

### 多应用程序高级用法

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

    // 创建多个应用程序
    webApp := &WebServerApp{addr: ":8080"}
    worker := &BackgroundWorker{interval: time.Second * 5}
    scheduler := &TaskScheduler{}

    // 创建带多个应用程序的服务器
    server, err := tserver.NewServer(ctx, tserver.WithApps(
        webApp, worker, scheduler,
    ))
    if err != nil {
        log.Fatal(err)
    }

    // 添加启动钩子
    server.AddStartHook(tserver.HookFunc(func(ctx context.Context) error {
        log.Println("加载配置...")
        return loadConfig()
    }))

    server.AddStartHook(tserver.HookFunc(func(ctx context.Context) error {
        log.Println("初始化数据库...")
        return initDatabase()
    }))

    // 添加关闭钩子
    server.AddStopHook(tserver.HookFunc(func(ctx context.Context) error {
        log.Println("保存应用程序状态...")
        return saveState()
    }))

    // 启动服务器
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }

    log.Println("服务器启动成功！")
    log.Println("按 Ctrl+C 停止...")

    // 等待关闭
    if err := server.Wait(ctx); err != nil {
        log.Printf("服务器错误: %v", err)
    }

    log.Println("所有应用程序已优雅停止")
}
```

## 核心接口

### App 接口

所有应用程序必须实现 `App` 接口：

```go
type App interface {
    // Run 启动应用程序并阻塞直到上下文被取消
    Run(ctx context.Context) error

    // Shutdown 在超时时间内执行优雅关闭
    Shutdown(ctx context.Context) error
}
```

### Hook 接口

生命周期钩子实现 `Hook` 接口：

```go
type Hook interface {
    // Run 执行钩子逻辑
    Run(ctx context.Context) error
}

// HookFunc 允许将普通函数用作钩子
type HookFunc func(ctx context.Context) error

func (h HookFunc) Run(ctx context.Context) error {
    return h(ctx)
}
```

## API 参考

### 服务器创建

#### `NewServer(ctx context.Context, options ...Option) (*Server, error)`

使用给定的上下文和选项创建新的服务器实例。

**选项：**

- `WithApp(app App)` - 添加单个应用程序
- `WithApps(apps ...App)` - 添加多个应用程序

```go
// 单个应用程序
server, err := tserver.NewServer(ctx, tserver.WithApp(myApp))

// 多个应用程序
server, err := tserver.NewServer(ctx, tserver.WithApps(app1, app2, app3))
```

### 服务器管理

#### `Start() error`

启动服务器和所有注册的应用程序。此方法：

1. 按顺序执行所有启动钩子
2. 启动信号处理
3. 并发启动所有应用程序
4. 标记服务器为运行状态

```go
if err := server.Start(); err != nil {
    log.Fatal("启动服务器失败:", err)
}
```

#### `Stop(ctx context.Context) error`

启动服务器的优雅关闭。此方法：

1. 并发关闭所有应用程序
2. 按顺序执行所有关闭钩子
3. 取消服务器上下文
4. 关闭完成通道

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := server.Stop(ctx); err != nil {
    log.Printf("停止错误: %v", err)
}
```

#### `Wait(ctx context.Context) error`

阻塞直到服务器停止或上下文被取消。

```go
// 无限等待
if err := server.Wait(context.Background()); err != nil {
    log.Printf("等待错误: %v", err)
}

// 带超时的等待
ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
defer cancel()

if err := server.Wait(ctx); err != nil {
    if err == context.DeadlineExceeded {
        log.Println("服务器未在超时时间内停止")
    } else {
        log.Printf("等待错误: %v", err)
    }
}
```

### 动态管理

#### `AddApp(apps ...App)`

动态添加应用程序到服务器（线程安全）。

```go
server.AddApp(newApp1, newApp2)
```

#### `GetApps() []App`

获取所有管理的应用程序的副本（线程安全）。

```go
apps := server.GetApps()
fmt.Printf("管理 %d 个应用程序\n", len(apps))
```

#### `IsRunning() bool`

检查服务器是否正在运行（线程安全）。

```go
if server.IsRunning() {
    log.Println("服务器正在运行")
}
```

### 生命周期钩子

#### `AddStartHook(hooks ...Hook)`

添加在服务器启动期间执行的钩子。

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

添加在服务器关闭期间执行的钩子。

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

### 信号处理

#### `AddStopSignal(signals ...os.Signal)`

添加触发优雅关闭的自定义信号。

```go
server.AddStopSignal(syscall.SIGUSR1, syscall.SIGUSR2)
```

**默认信号：** `SIGINT`, `SIGTERM`

## 示例

### HTTP 服务器应用程序

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

### 后台工作器应用程序

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
                log.Printf("工作错误: %v", err)
            }
        }
    }
}

func (w *BackgroundWorker) Shutdown(ctx context.Context) error {
    log.Printf("工作器 %s 正在关闭...", w.name)
    return nil
}

func (w *BackgroundWorker) doWork(ctx context.Context) error {
    // 模拟工作
    log.Printf("工作器 %s 正在执行任务...", w.name)
    time.Sleep(100 * time.Millisecond)
    return nil
}
```

## 最佳实践

### 1. 上下文处理

在应用程序中始终尊重上下文取消：

```go
func (a *MyApp) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err() // 尊重取消
        default:
            // 执行工作
        }
    }
}
```

### 2. 错误处理

在应用程序和钩子中优雅地处理错误：

```go
func (a *MyApp) Run(ctx context.Context) error {
    if err := a.initialize(); err != nil {
        return fmt.Errorf("初始化失败: %w", err)
    }

    // 主应用程序循环
    return a.mainLoop(ctx)
}
```

### 3. 资源清理

始终在 Shutdown 方法中清理资源：

```go
func (a *MyApp) Shutdown(ctx context.Context) error {
    // 关闭连接、文件等
    if a.db != nil {
        return a.db.Close()
    }
    return nil
}
```

### 4. 超时管理

为关闭操作使用适当的超时：

```go
func main() {
    // ... 服务器设置 ...

    if err := server.Start(); err != nil {
        log.Fatal(err)
    }

    // 使用合理的超时等待
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := server.Wait(ctx); err != nil {
        log.Printf("关闭超时: %v", err)
    }
}
```

## 错误处理

TServer 优雅地处理错误：

- **启动钩子失败**：如果任何启动钩子失败，服务器启动将被中止
- **应用程序错误**：应用程序运行时错误会被记录，但不会停止其他应用程序
- **停止钩子失败**：停止钩子错误会被记录，但不会阻止关闭完成
- **关闭错误**：应用程序关闭错误会被记录，但不会中断关闭过程

## 线程安全

所有服务器操作都是线程安全的：

- `AddApp()`, `GetApps()`, `IsRunning()` 可以并发调用
- 钩子管理由互斥锁保护
- 服务器状态更改是原子的

## 测试

该包包含 98.1% 覆盖率的全面测试：

```bash
go test -v
go test -cover
go test -bench=.
```

## 贡献

1. Fork 仓库
2. 创建您的功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交您的更改 (`git commit -am 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 打开一个 Pull Request

## 许可证

该项目在 MIT 许可证下授权 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 更新日志

### v1.0.0

- 初始版本发布
- 多应用程序管理
- 生命周期钩子支持
- 优雅关闭
- 信号处理
- 基于上下文的控制
- 全面的测试套件

---

有关更多示例和详细文档，请查看 [example](example/) 目录。
