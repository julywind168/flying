# Flying

Flying 是一个用 Go 语言编写的事件调度框架, flying/berry 是基于 flying 实现的轻量级游戏服务器框架

## 特性

- 支持异步事件驱动的实体调度系统
- 内置 Pub/Sub 系统用于事件广播
- 支持实体生命周期管理和依赖注入
- 提供指标收集和慢事件检测
- 支持优雅关闭和资源清理

## 快速开始

1. 启动演示服务器:

```bash
go run demo/server/main.go
```

2. 运行演示客户端:

```bash 
go run demo/client/main.go
```

## 项目结构

```
.
├── berry/          # 游戏服务器框架
├── demo/           # 示例代码和演示服务器
│   ├── client/     # 演示客户端
│   ├── common/     # 共享代码(协议、模型等)
│   ├── config/     # 配置
│   └── server/     # 演示服务器实现
└── flying_*.go     # 核心代码
```

## 配置

主要配置项包括:

1. 数据库配置 ([db.go](demo/common/db/db.go))
2. JWT 密钥配置 ([config.go](demo/config/config.go))

## 开发

1. 添加新的实体:
```go
type YourEntity struct {
    id string
}

func (e *YourEntity) ID() string { return e.id }
func (e *YourEntity) Lockfree() bool { return false }
func (e *YourEntity) Dependencies() []string { return nil }
```

2. 注册实体:
```go
scheduler.RegisterEntity(&YourEntity{id: "entity1"})
```

3. 发送事件:
```go
scheduler.SubmitAsync(&flying.Event{
    EntityID: "entity1",
    Handler: func(ctx flying.Context, e flying.Entity) error {
        // 处理逻辑
        return nil
    },
})
```

## 许可证

本项目采用 Apache License 2.0 许可证 - 查看 [LICENSE](LICENSE) 文件了解更多细节。