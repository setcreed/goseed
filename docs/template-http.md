# 默认 HTTP 模板契约

本文件定义 D2 的实现与验收边界。当前完成状态见 ROADMAP；服务命令在实际实现后写入渲染项目 README。

## 源码与验证

模板源码是唯一维护入口，渲染项目不导入 goseed。Go 固定为 1.26.4，运行依赖使用已核验版本；root CLI 的项目生成命令在 D5 接入。渲染与端到端测试使用项目 `.tmp/d2/` 下本次独立目录。

目录沿用架构文档中的 Handler → Biz → Store；所有可写模型字段由转换代码显式选择。普通构造函数传递依赖，do 仅在 Bootstrap 注册和调用。

## 服务与开发命令

```sh
make build
./bin/apiserver dev init
./bin/apiserver migrate up
./bin/apiserver dev token --subject alice --ttl 15m
./bin/apiserver serve
```

- 默认命令为 `serve`；各服务命令接受 `--config`，默认路径为 `configs/app.yaml`。`serve` 支持显式 `--address` 和 `--env`，覆盖配置值。
- `dev init` 创建权限为 0600 的开发配置和密码学随机 32 字节 JWT 密钥。已有有效开发配置保留原密钥；生产配置、符号链接及无效配置报错，不覆盖。生成模板本身仅包含无密钥的配置示例。
- `dev token` 仅在有效开发配置下向标准输出返回 Token，主体必填且不含首尾空白；TTL 至少 1s 且不超过配置的 JWT TTL，JWT 时间精度为秒。不提供 HTTP 签发接口。
- `migrate up` 显式应用嵌入的 SQLite SQL 迁移；`migrate status` 读取当前迁移版本。迁移失败返回非零退出码，不记录 SQL 参数或密钥。
- `make dev` 依次执行开发初始化、迁移和启动；生产环境禁用开发初始化及开发签发。服务启动验证 schema 已就绪，不自动建表或执行迁移。

## 配置

覆盖顺序：内置默认值 < YAML 文件 < 已绑定环境变量 < 显式命令行参数。环境变量使用 `APP_` 前缀，例如 `APP_JWT_SECRET`、`APP_HTTP_ADDRESS`、`APP_DATABASE_PATH`。显式配置文件缺失报错；默认文件缺失时，全部必需配置可由环境变量提供。

配置包含 `env`、`http`、`log`、`database`、`jwt`：

- 默认运行环境为 `production`；开发初始化显式写入 `development`。生产日志为 JSON，开发配置使用 console；时间为 UTC，日志不包含配置全文、Token、正文、SQL 参数或 URL 查询。
- HTTP 默认地址 `127.0.0.1:8080`；限制请求体、请求预算、ReadHeader/Read/Write/Idle 超时、Header 大小及关闭预算。CORS 仅允许配置的确切来源。TLS 证书与私钥必须成对提供。pprof 仅允许开发环境的回环监听地址，默认关闭。
- SQLite 默认路径 `data/app.db`；每个连接开启外键、WAL 和 busy timeout。连接池和慢查询阈值有明确范围，生产日志中的 GORM 查询不打印原始 SQL 或参数。
- JWT 固定 HS256；`secret` 为 Base64 编码且解码后至少 32 字节，`issuer`、`audience` 非空，TTL 默认 15m、最大 1h。生产没有默认密钥，缺少配置时启动失败。
- 未知字段、错误类型、无单位 Duration、非法枚举、缺失凭据、无效地址及相互矛盾的超时配置都报错；错误不包含配置值中的秘密。

## HTTP 与认证

公开入口为 `GET /healthz` 与 `GET /readyz`；后者检查数据库并在关闭开始后不就绪。示例业务 `/v1/examples` 全部要求 `Authorization: Bearer <token>`。

请求 ID 接受 1 至 64 位字母、数字、点、下划线和连字符；其他值重新生成，响应通过 `X-Request-ID` 返回。请求 Context 贯穿 Handler、Biz、Store；超时、取消和 panic 经统一错误边界处理。恢复日志不输出 panic 值或请求头。

HTTP 标准库的协议与握手错误也接入配置的日志编码器，只记录稳定事件，不转发可能包含对端详情、panic 值或堆栈的原始文本。

JWT 解析前限制长度并校验三段 Base64URL 字符；要求有效签名、issuer、audience、subject、exp、iat、nbf，校验到期、尚未生效、未来签发与最大生命周期。身份使用私有 Context 键传递，不信任正文中的所有者字段。

成功返回资源 JSON，列表返回 `items`、`total`、`page`、`page_size`；错误返回 `code`、`message`、`request_id`。认证失败为 401，输入错误为 400，不存在或不属于当前主体的资源为 404，唯一冲突为 409，超时为 504，未分类内部错误为 500。响应不包含 SQL、堆栈或内部地址。

## 示例 CRUD

资源字段为自增 `id`、服务端决定的 `owner`、`name`、`description`、`enabled`、UTC `created_at` 与 `updated_at`。名称为 1 至 80 个 Unicode 字符，不允许纯空白或首尾空白；描述最多 1000 个字符；enabled 创建时默认 true。唯一约束为 `(owner, name)`。

- `POST /v1/examples`：创建，成功为 201。
- `GET /v1/examples/{id}`：读取当前主体资源。
- `GET /v1/examples?page=1&page_size=20`：仅列出当前主体资源，page 上限 100000、page_size 上限 100，按 id 升序。
- `PATCH /v1/examples/{id}`：按字段存在性更新，允许 description 为空、enabled 为 false；空更新、null、保留字段和未知字段拒绝。
- `DELETE /v1/examples/{id}`：删除，成功为 204；不存在为 404。

所有写入拒绝未知字段、重复 JSON 键、null、额外 JSON 值、无效 UTF-8 和超限请求体。业务身份与资源归属由 Biz/Store 检查，不能靠客户端自行约束。数据库内部异常在 Store 转为稳定业务错误；约束始终由数据库兜底。

## 验收

渲染结果经过 readonly 构建、测试、vet、lint 和 race；不依赖 goseed 源码或 `.tmp/` 输入。进程级验收覆盖开发初始化保留密钥、显式迁移与重复执行、生产禁用开发命令、配置覆盖与未知键、缺失 schema、非法 TLS、端口占用、认证与 CRUD、跨主体隔离、分页、零值更新、非法正文、请求取消及重启持久化。

Store 层以同一事务连接创建两个仓储写入，主动失败后验证整体回滚；不把 GORM 对象暴露给 Biz。生命周期测试覆盖启动失败资源关闭、就绪转变、请求排空与关闭到期强停；日志和错误输出核对测试秘密、Token 与 SQL 数据均不出现。

交付包含配置示例、项目规范与使用文档、Makefile、SQLite 迁移、Swagger 2.0 接口文档及 Dockerfile。容量、备份恢复、多实例和具体业务上线评估不由本检查点代替。

容器验证入口 `make template-container`：实际渲染独立项目，使用固定摘要的官方 Go 1.26.4 镜像构建，以非 root 运行，使用本次专用数据卷和随机回环端口验证显式迁移、生产禁用签发、CRUD 及容器重启持久化，结束清理本次容器、卷与测试镜像。常规 `make check` 不自动启动 Docker。
