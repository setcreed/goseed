# Gateway 模板契约

`Options.Transport: "gateway"` 选择 HTTP → gRPC Handler → Biz 的真实转发模式；保留直接 gRPC 入口，不生成 Gin、HTTP 业务 Handler 或重复 Biz。四种存储与 Redis／Casbin 开关组合，共 16 种。公开创建 CLI 属 D5，当前状态见 [ROADMAP](../ROADMAP.md) 。

## 协议与输入

SQL 使用 `/v1/examples` 的 POST／GET，以及 `/v1/examples/{id}` 的 GET／PATCH／DELETE；无数据库仅 GET `/v1/examples?name=...`。映射放在 Proto 同目录的固定 HTTP 配置文件，Swagger 安全声明位于同目录的 `openapi.yaml`；两份配置由固定版本插件读取，普通构建使用已生成 Go 和 Swagger 2.0。显式生成不修改业务文件或依赖清单。

成功响应采用 ProtoJSON，字段使用 Proto 的 snake_case，64 位整数为字符串；所有成功方法返回 200，Delete 返回 `{}`。PATCH 保留 optional 的零值语义。请求拒绝未知／重复／null 字段、额外 JSON、非法 UTF-8、非法／重复／未知 query、正文超限及正文中的路径 ID；路径 ID 只取 URL。POST／PATCH 只接受 JSON。不启用 HTTP 方法覆盖或表单回退。错误采用 Gateway 的数字 gRPC code、脱敏 message 与空 details，HTTP 状态按标准 gRPC 映射；请求 ID 在响应头提供。

只转发 Authorization 与合法 X-Request-ID，拒绝重复认证头，忽略客户端伪造的 Grpc-Metadata-* 身份。JWT → Casbin → Biz／缓存在 RPC 入口执行；HTTP Context、取消和预算传入 RPC，不接受客户端无限延长预算。访问日志只记方法、固定路由、状态、请求 ID 与耗时，不含 Token、正文、query 或动态路径。

## 配置与生命周期

HTTP 默认 `127.0.0.1:8080`，gRPC 默认 `127.0.0.1:9090`；`serve --address` 与 `--grpc-address` 分别覆盖。沿用两侧预算、TLS、CORS、健康和开发 pprof 约定。新增 `gateway.ca_file`、`gateway.server_name` 仅用于本进程 Gateway 到本进程 RPC 的证书验证；生产 gRPC 必须 TLS，绝不跳过证书校验。客户端目标由实际监听地址确定，通配监听转为回环，不提供任意上游地址或独立外部服务模式。

先绑定两端口，启动 RPC，再在有限启动预算内完成健康 RPC 和 TLS 验证，最后开放 HTTP。任一启动失败回收监听器、RPC、转发连接和共享依赖；不自动迁移。正常退出先停止两侧就绪，排空 HTTP，HTTP 返回后才关闭转发连接、排空 RPC，最后关闭共享依赖。关闭总预算为两侧预算之和。RPC 意外返回时同样回收 HTTP，允许其在预算内失败退出。

## 验收

16 种组合 readonly 编译、Linux 纯 Go 构建；SQLite／无数据库代表项目 test、vet、race、lint、实际 HTTP／RPC、Proto／Gateway／Swagger 逐字节复现。验证认证授权先于缓存、跨协议 CRUD／问候、零值更新、输入边界、错误脱敏、请求 ID、预算取消、端口占用、证书失败、失败回收和顺序排空。真实 MySQL／PostgreSQL、TLS Redis 和双端口生产容器复用隔离夹具，无外部用例跳过。

业务规则及协议边界见 [gRPC](template-grpc.md) 与 [可选模板](template-options.md) 。实现采用 [官方外部 HTTP 映射方式](https://grpc-ecosystem.github.io/grpc-gateway/docs/mapping/grpc_api_configuration/) 与 [Gateway 定制接口](https://grpc-ecosystem.github.io/grpc-gateway/docs/mapping/customizing_your_gateway/) 。
