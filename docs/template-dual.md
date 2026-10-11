# dual 模板契约

D4 的本次功能交付 `Options.Transport: "dual"`。Gin HTTP 与 gRPC 使用独立 Handler，共用同一个 Biz 实例、Store、数据库池、JWT、可选 Redis 和 Casbin；HTTP 不转发到 gRPC。Gateway 留在下一功能，公开生成 CLI 仍属 D5。

## 配置与源码

同时保留 HTTP、Swagger 2.0、Proto、公开 `pkg/api`、gRPC 示例客户端及固定版本显式协议生成入口。HTTP 与 gRPC 分别使用现有协议配置与安全链，生产 gRPC 必须 TLS，Reflection 只允许开发回环并认证；HTTP 的 TLS、CORS、pprof 规则保持不变。

默认 HTTP 地址 `127.0.0.1:8080`，gRPC 地址 `127.0.0.1:9090`。`serve --address` 只覆盖 HTTP，`serve --grpc-address` 只覆盖 gRPC；环境变量分别为 `APP_HTTP_ADDRESS` 与 `APP_GRPC_ADDRESS`。两个显式监听地址相同且端口非零时配置校验拒绝；随机端口零可用于隔离测试。开发、迁移及策略命令不接受监听地址覆盖。

Redis 命令预算必须小于两个协议的请求预算。所有业务入口按 JWT → Casbin → Biz／缓存执行，资源归属与输入规则由共享 Biz 保证；协议特有错误结构和字段类型保持各自契约，不要求 HTTP JSON 与 Proto JSON 逐字节相同。

## 生命周期

启动先绑定两个监听器，任何监听失败都关闭本次已绑定监听器，不启动单独可用的半个服务。证书或依赖初始化失败关闭已创建资源，服务启动不自动迁移。

两个服务并发运行；收到退出信号或任一服务返回时，先停止两侧就绪，再取消两侧服务生命周期。HTTP 与 gRPC 并发排空，各使用其配置的关闭预算，整体排空预算为二者较大值，而非两者相加；超时分别强制关闭连接／RPC。两个 Serve 均返回后再统一关闭缓存、数据库和日志。健康 Watch 和开发 Reflection 遵循纯 gRPC 规则。

## 验收

四种存储 × Redis 开关 × Casbin 开关共 16 种 dual 组合均需生成、readonly 编译及 Linux 纯 Go 构建。SQLite／无数据库代表组合完成 test、vet、race、lint、实际 HTTP／RPC 调用与 Proto 复现。

覆盖跨协议创建、读取、零值更新、删除、分页／文本边界、JWT 失败、主体与角色隔离、权限撤销先于缓存、请求 ID、取消与 deadline；验证两个端口分别占用、证书失败、启动清理、依赖关闭、同时排空、强制关闭、任一 Serve 异常和重启持久化。真实 MySQL／PostgreSQL、TLS Redis 与生产双端口容器使用现有隔离夹具，不触及已有实例。

现有协议契约见 [HTTP](template-http.md) 与 [gRPC](template-grpc.md) ，缓存和授权见 [可选模板](template-options.md) 。
