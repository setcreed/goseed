# 纯 gRPC 模板契约

`Options.Transport` 支持 `http`（默认）、`grpc` 与 `dual`；dual 约束见 [双协议模板](template-dual.md) ，Gateway 属后续功能，CLI 写入仍属 D5。复用 Biz、Store、JWT、Casbin 与 Redis，纯 gRPC 不交付 Gin、HTTP 配置／中间件、Swagger 或业务 HTTP 服务。

Proto 源位于 `api/proto/example/v1/`，已生成代码位于 `pkg/api/example/v1/`，供其他 Go 服务导入协议类型；Handler 位于 `internal/apiserver/handler/grpc/example/`，服务及拦截器位于 `internal/pkg/rpc/`。提供示例客户端和固定版本的显式协议生成入口，普通构建不运行 protoc、不安装工具或 tidy。四种存储 × Redis 开关 × Casbin 开关共 16 种组合均需 readonly 编译，代表组合完成实际调用。

资源模块各自使用 `api/proto/<module>/v1/<module>.proto` 与 `pkg/api/<module>/v1/`。`scripts/proto.py` 按模块路径排序并复现所有模块协议，检查 protoc 与插件固定版本；新资源的 HTTP、RPC 与 dual 渲染共用一次 Store/Biz 组装，dual 的两种 Handler 接收同一个 Biz。资源 CRUD 使用独立 `<Name>Service`，权限目录按每个服务的 FullMethod 精确映射；创建必需字段与 PATCH 更新字段采用 Proto3 optional 保留出现性，不暴露 owner。

SQL 提供 CRUD，无数据库提供认证问候。Proto3 optional 保留 PATCH 字段存在性和零值；分页省略取 1/20，显式零由 Biz 拒绝；ID 限 1 至 int64 最大值。拒绝未知 Proto 字段，文本、分页和资源归属由同一 Biz 校验。

## 配置与安全

强类型 grpc 配置：address、request_timeout、shutdown_timeout、max_recv_bytes、max_send_bytes、max_header_bytes、max_concurrent_streams、tls_cert、tls_key、reflection。默认回环 `127.0.0.1:9090`、请求 5s、关闭 10s、接收 1MiB、发送 2MiB、Header 64KiB、每连接并发 128、Reflection 关闭。环境绑定 `APP_GRPC_*`，`serve --address` 覆盖当前协议。时长为带单位字符串，限 1ms 至 10m；消息上限 1 至 16MiB，Header 1KiB 至 4MiB，并发 1 至 1024。证书／私钥成对，TLS 最低 1.2；生产要求 TLS，开发可明文。Reflection 仅开发回环可开启，并要求 JWT。

业务 unary 要求单个 `authorization: Bearer <token>`，JWT 主体进入 Context。Casbin 在缓存／Biz 前按精确 FullMethod 映射 example 的 read/create/update/delete，无数据库只有 read。未知方法不获授权。请求 ID 采用单个 `x-request-id`，仅接受 1 至 64 个 ASCII 字母、数字、点号、下划线或连字符；省略时随机生成，响应回传，日志与 Store 共用 Context。日志仅输出已登记方法、请求 ID、Status 与耗时，不输出 Token、metadata、正文、主体或原始 cause；panic 脱敏为 Internal。

保留调用方更短的 deadline 与取消。错误映射为 InvalidArgument、Unauthenticated、PermissionDenied、NotFound、AlreadyExists、Unavailable、Canceled、DeadlineExceeded 和 Internal，内部原因不透传；消息大小由 transport 限制。

## 健康与退出

标准 Health Check／Watch 为显式公共入口。空服务名和业务服务名表示依赖就绪，`liveness` 表示存活；未知服务名 NotFound。Check 当次校验依赖，Watch 最多每 500ms 更新，依赖预算 200ms；可选 Redis 故障不阻断，必需 Redis／数据库故障为 NOT_SERVING，恢复后 SERVING。关闭先停止就绪、通知 Watch，再 GracefulStop；超时强制 Stop 并取消请求，等 Serve 返回后关闭池。

不生成业务流。Stream 拦截器仅放行 Health Watch，以及已显式开启且通过 JWT 的 Reflection；其他流式入口拒绝。证书或监听失败关闭已初始化资源，不自动迁移。

## 验收与依据

验证 16 组合裁剪、固定清单、Linux 纯 Go 构建；代表产物执行 test/vet/race/lint 与真实 RPC，覆盖 CRUD／问候、零值／分页、归属与角色隔离、未知字段、JWT/Casbin、请求 ID、取消/deadline、panic／原始错误脱敏、消息限制、健康 Watch／故障恢复、优雅与强制退出、TLS 信任／主机名、Reflection、启动失败、持久化及日志脱敏。实际执行交付的 Proto 生成脚本逐字节比对产物，并用独立 Go 模块导入 `pkg/api/example/v1`；新增 RPC 的权限登记遵循安全链要求。真实 SQL／Redis 沿用隔离夹具，禁止跳过或触及已有实例；生产 TLS 容器验证非 root、只读根文件系统与证书／策略只读挂载。

依据：[Health 文档](https://grpc.io/docs/guides/health-checking/) 、[优雅关闭文档](https://grpc.io/docs/guides/server-graceful-stop/) 、[Reflection 文档](https://grpc.io/docs/guides/reflection/) 。固定版本及维护证据沿用本日已核验的 [依赖方案](dependencies.md) 。
