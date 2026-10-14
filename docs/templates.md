# 生成项目模板指南

本文集中说明生成项目的运行、配置与验收契约。命令参数见 [CLI 契约](cli.md)，目录与组件边界见 [技术架构](architecture.md)，单机部署见 [部署契约](deployment.md)；当前状态以 [ROADMAP](../ROADMAP.md) 为准。

## 导航

- [默认 HTTP 模板](#默认-http-模板)
- [数据库与可选能力](#数据库与可选能力)
- [纯 gRPC 模板](#纯-grpc-模板)
- [dual 模板](#dual-模板)
- [Gateway 模板](#gateway-模板)

## 默认 HTTP 模板

本节说明默认 HTTP 项目的运行、配置与验收边界。服务命令以生成项目 README 和实际产物为准，当前状态见 ROADMAP。

### 源码与验证

模板源码是唯一维护入口，渲染项目不导入 goseed。Go 最低版本为 1.26.4，写入生成项目的 `go.mod`；运行依赖使用已核验版本。渲染与端到端测试使用项目 `.tmp/d2/` 下本次独立目录。

目录沿用架构文档中的 Handler → Biz → Store；所有可写模型字段由转换代码显式选择。普通构造函数传递依赖，do 仅在 Bootstrap 注册和调用。

### 服务与开发命令

```sh
make build
./bin/apiserver dev init
./bin/apiserver migrate up
./bin/apiserver dev token --subject alice --ttl 15m
./bin/apiserver serve
```

- 默认命令为 `serve`；各服务命令接受 `--config`，默认路径为 `configs/app.yaml`。`serve` 支持显式 `--address` 和 `--env`，覆盖配置值。
- `dev init` 创建权限为 0600 的开发配置和密码学随机 32 字节 JWT 密钥。已有有效开发配置保留原密钥；生产配置、符号链接及无效配置报错，不覆盖。生成模板本身仅包含无密钥的配置示例。
- `dev token` 仅在有效开发配置下向标准输出返回 Token，主体为 1 至 128 个 Unicode 字符且不含首尾空白或控制字符；TTL 至少 1s 且不超过配置的 JWT TTL，JWT 时间精度为秒。不提供 HTTP 签发接口。
- `migrate up` 显式应用嵌入的 SQLite SQL 迁移；`migrate status` 读取当前迁移版本。迁移失败返回非零退出码，不记录 SQL 参数或密钥。
- `make dev` 依次执行开发初始化、迁移和启动；生产环境禁用开发初始化及开发签发。服务启动验证 schema 已就绪，不自动建表或执行迁移。

### 配置

覆盖顺序：内置默认值 < YAML 文件 < 已绑定环境变量 < 显式命令行参数。环境变量使用 `APP_` 前缀，例如 `APP_JWT_SECRET`、`APP_HTTP_ADDRESS`、`APP_DATABASE_PATH`。显式配置文件缺失报错；默认文件缺失时，全部必需配置可由环境变量提供。

配置包含 `env`、`http`、`log`、`database`、`jwt`：

- 默认运行环境为 `production`；开发初始化显式写入 `development`。生产日志为 JSON，开发配置使用 console；时间为 UTC，日志不包含配置全文、Token、正文、SQL 参数或 URL 查询。
- HTTP 默认地址 `127.0.0.1:8080`；限制请求体、请求预算、ReadHeader/Read/Write/Idle 超时、Header 大小及关闭预算。CORS 仅允许配置的确切来源。TLS 证书与私钥必须成对提供。pprof 仅允许开发环境的回环监听地址，默认关闭。
- SQLite 默认路径 `data/app.db`；每个连接开启外键、WAL 和 busy timeout。连接池和慢查询阈值有明确范围，生产日志中的 GORM 查询不打印原始 SQL 或参数。
- JWT 固定 HS256；`secret` 为 Base64 编码且解码后至少 32 字节，`issuer`、`audience` 非空，TTL 默认 15m、最大 1h。生产没有默认密钥，缺少配置时启动失败。
- 未知字段、错误类型、无单位 Duration、非法枚举、缺失凭据、无效地址及相互矛盾的超时配置都报错；错误不包含配置值中的秘密。

### HTTP 与认证

公开入口为 `GET /healthz` 与 `GET /readyz`；后者检查数据库并在关闭开始后不就绪。示例业务 `/v1/examples` 全部要求 `Authorization: Bearer <token>`。

请求 ID 接受 1 至 64 位字母、数字、点、下划线和连字符；其他值重新生成，响应通过 `X-Request-ID` 返回。请求 Context 贯穿 Handler、Biz、Store；超时、取消和 panic 经统一错误边界处理。恢复日志不输出 panic 值或请求头。

HTTP 标准库的协议与握手错误也接入配置的日志编码器，只记录稳定事件，不转发可能包含对端详情、panic 值或堆栈的原始文本。

JWT 解析前限制长度并校验三段 Base64URL 字符；要求有效签名、issuer、audience、subject、exp、iat、nbf，校验到期、尚未生效、未来签发与最大生命周期。身份使用私有 Context 键传递，不信任正文中的所有者字段。

成功返回资源 JSON，列表返回 `items`、`total`、`page`、`page_size`；错误返回 `code`、`message`、`request_id`。认证失败为 401，输入错误为 400，不存在或不属于当前主体的资源为 404，唯一冲突为 409，超时为 504，未分类内部错误为 500。响应不包含 SQL、堆栈或内部地址。

### 示例 CRUD

资源字段为自增 `id`、服务端决定的 `owner`、`name`、`description`、`enabled`、UTC `created_at` 与 `updated_at`。名称为 1 至 80 个 Unicode 字符，不允许纯空白或首尾空白；描述最多 1000 个字符，文本拒绝 NUL；enabled 创建时默认 true。唯一约束为 `(owner, name)`，区分大小写。

- `POST /v1/examples`：创建，成功为 201。
- `GET /v1/examples/{id}`：读取当前主体资源。
- `GET /v1/examples?page=1&page_size=20`：仅列出当前主体资源，page 上限 100000、page_size 上限 100，按 id 升序。
- `PATCH /v1/examples/{id}`：按字段存在性更新，允许 description 为空、enabled 为 false；空更新、null、保留字段和未知字段拒绝。
- `DELETE /v1/examples/{id}`：删除，成功为 204；不存在为 404。

所有写入拒绝未知字段、重复 JSON 键、null、额外 JSON 值、无效 UTF-8 和超限请求体。业务身份与资源归属由 Biz/Store 检查，不能靠客户端自行约束。数据库内部异常在 Store 转为稳定业务错误；约束始终由数据库兜底。

### 验收

渲染结果经过 readonly 构建、测试、vet、lint 和 race；不依赖 goseed 源码或 `.tmp/` 输入。进程级验收覆盖开发初始化保留密钥、显式迁移与重复执行、生产禁用开发命令、配置覆盖与未知键、缺失 schema、非法 TLS、端口占用、认证与 CRUD、跨主体隔离、分页、零值更新、非法正文、请求取消及重启持久化。

Store 层以同一事务连接创建两个仓储写入，主动失败后验证整体回滚；不把 GORM 对象暴露给 Biz。生命周期测试覆盖启动失败资源关闭、就绪转变、请求排空与关闭到期强停；日志和错误输出核对测试秘密、Token 与 SQL 数据均不出现。

交付包含配置示例、项目规范与使用文档、Makefile、SQLite 迁移、Swagger 2.0 接口文档及 Dockerfile。容量、备份恢复、多实例和具体业务上线评估不由本检查点代替。

容器验证入口 `make template-container`：实际渲染独立项目，使用固定摘要的官方 Go 1.26.4 镜像构建，以非 root 运行，使用本次专用数据卷和随机回环端口验证显式迁移、生产禁用签发、CRUD 及容器重启持久化，结束清理本次容器、卷与测试镜像。常规 `make check` 不自动启动 Docker。

## 数据库与可选能力

本节说明数据库选择、无数据库模式及 Redis/Casbin 的组合与验收约定。生成参数见 [CLI 契约](cli.md)，当前交付状态以 ROADMAP 为准。

### SQL 数据库变体

内部 `templates.Render(Project, Options)` 返回完整文件计划；`Options.Database` 为空时采用 SQLite，数据库枚举为 `sqlite`、`mysql`、`postgres`、`none`。未知枚举直接拒绝，不生成部分文件。`Default` 保持默认模板语义。

选择发生在生成阶段：产物只包含对应驱动、配置字段、SQL 方言、测试与使用文档，不在运行时切换数据库。共用 Handler → Biz → Store、JWT、错误和生命周期。依赖清单按变体固定，构建使用 readonly；渲染不联网、不 tidy、不写文件。

SQLite 保持文件、连接 PRAGMA 和单实例约定。MySQL、PostgreSQL 采用强类型 host/port/user/password/name，不接受自由 DSN；默认端口分别为 3306、5432，用户及数据库名默认项目名，密码无默认值。连接池默认最多 4 个连接、2 个空闲连接，连接寿命 30m、空闲寿命 5m、连接预算 5s，慢查询阈值 200ms。连接预算限制在 1s 至 30s，池寿命必须为正且空闲寿命不得超过连接寿命。

外部数据库默认 `database.tls: true`，TLS 至少 1.2，校验服务端证书及 host；可通过 `database.ca_file` 增加私有 CA。禁止跳过验证及 TLS 回退。只有 development 允许 tls 为 false；关闭 TLS 时不能设置 CA。错误与日志不输出密码、连接串、SQL 和参数。MySQL 使用独立 Connector，PostgreSQL 使用独立 pgx 配置，共用现有连接池，不注册全局 TLS 配置。

JWT 主体限制为 1 至 128 个 Unicode 字符，拒绝控制字符及首尾空白，三个数据库保持一致的资源归属与名称唯一语义。MySQL 使用 utf8mb4 二进制排序规则；PostgreSQL 文本使用 C 排序规则。所有读写时间使用 UTC。MySQL 更新使用匹配行数，重复写入相同值不能把已有资源误报为 404。

显式 `migrate status/up` 共用命令；status 和启动检查不创建 Goose 表。期望版本从嵌入 SQL 推导；status 只显示已应用最大版本，启动核对完整已应用集合及实际列，up 只接受有序前缀，拒绝未知版本和中间断档，不自动修复历史。就绪依赖 Ping。资源 schema 原型与方言草稿的约束见 [资源接入方案](resource-generation.md#迁移版本与资源-schema) ，完整 CRUD/CLI 状态见 ROADMAP。SQLite/PostgreSQL 迁移使用方言支持的事务；MySQL DDL 不具备事务回滚，迁移失败后必须核对实际 schema，不通过自动重试修补部分 schema。退出和启动失败均关闭连接池。

外部数据库的 `dev init` 只创建开发 JWT 密钥并设置开发明文连接；需要先由环境安全提供数据库密码，再执行保留已有配置、Token、迁移与服务命令。脚手架不部署数据库、不创建生产账号。

### 验收

普通检查对三个 SQL 变体执行实际渲染、readonly 测试、vet、race、lint 与构建，并核对驱动裁剪、清单不可变及文档链接。外部数据库未配置时，生成项目明确跳过需要真实服务的测试，不能据此声明数据库已验证。

`make template-db` 显式启动本次隔离的固定摘要 MySQL、PostgreSQL 容器、全新数据卷和随机回环端口。测试账号仅用于该次验证；测试夹具创建随机命名的独立数据库，结束关闭连接并仅删除自己创建的数据库。真实验收禁止跳过外部用例，覆盖空库只读检查、迁移重放、约束与大小写、事务回滚、零值更新、Context、数据库关闭、错误脱敏、HTTP 跨主体隔离与服务重启持久化。

数据库认证失败、TLS 不可信、无效 CA、取消、缺 schema、错误 schema 或池关闭均必须失败；启动失败不能泄露连接池。基础 CRUD 验证不代替业务上线前的容量、备份恢复和数据库权限评估。

### 无数据库变体

`Database: "none"` 在生成时移除数据库配置、ORM、数据库驱动、迁移、模型与 Store；目录 `variants/none/` 仅维护无数据库示例、必要测试、依赖清单和使用文档的替换模板，其余 HTTP/JWT 基础能力共用基础模板。生成项目不使用内存仓储模拟持久化。

示例采用 Handler → Biz，`GET /v1/examples` 必须通过 JWT 认证，返回 `{"message":"hello, world","subject":"alice"}`。可选 query `name` 默认 `world`，显式提供时为 1 至 80 个 Unicode 字符，拒绝无效 UTF-8、控制字符与首尾空白。Handler 拒绝未知或重复参数及错误百分号编码；Biz 校验输入、请求主体并传播取消或 deadline。没有资源 CRUD 路由、写入或重启持久化承诺。

保留配置优先级与强类型校验、开发密钥初始化/签发、HTTP TLS/CORS、日志脱敏、请求预算和优雅退出；就绪只反映服务生命周期。`migrate` 命令直接报错且不创建文件，帮助与 Makefile 不登记迁移；`make dev` 仅初始化并启动。旧数据库 YAML 字段作为未知配置拒绝。Docker 继续使用固定构建镜像与非 root scratch 运行，不创建或声明数据库数据目录。

验收以实际生成模块为对象：readonly 测试、vet、race、lint、本机构建与 Linux 纯 Go 构建；核对依赖树和交付文件不含 ORM/驱动/迁移，清单不变。真实进程无需数据库即可启动，覆盖 JWT、两个主体、输入边界、未登记方法/资源、重启、退出及日志脱敏。数据库模板继续执行原有回归。

### Redis 可选能力

`Options.Redis` 默认 false，可与四个数据库枚举组合。开启时才生成 `internal/pkg/cache/`、Biz 缓存接口与示例、配置、依赖、测试、Swagger 错误响应和 `docs/redis.md`。`variants/redis/common/` 保存共用客户端及缓存模板；各数据库子目录保存对应完整依赖清单。运行时不提供启停功能开关。缓存不改变 JWT 或 Casbin 存储，不包含集群、锁、队列或限流。

强类型配置为 address、username、password、db、key_prefix、ttl、command_timeout、dial_timeout、pool_size、required、tls、ca_file。默认地址 `127.0.0.1:6379`、DB 0、项目名 namespace、TTL 30s、命令预算 200ms、拨号上限 1s、池大小 4、required false、TLS true；无默认密码。TTL 限 1s 至 5m，命令预算限 1ms 至 5s 且小于 HTTP 请求预算，拨号限 1ms 至 5s，池大小 1 至 64，DB 0 至 15，namespace 只允许 1 至 64 个 ASCII 字母/数字/冒号/下划线/连字符。生产需要密码和验证证书的 TLS 1.2+，私有 CA 通过 ca_file 加入；开发初始化设置开发明文连接，不提供密码或部署 Redis。真实密钥和凭据不进入日志。

每个操作保留调用方 Context，另加短命令 deadline；明确读写/池等待上限、关闭自动重试与客户端缓存/实验流水线。提前取消立即拒绝，运行中纯取消最多等待短命令预算后返回调用方取消；不声称修复客户端上游的无 deadline 取消问题。

SQL 示例只缓存按主体与 ID 读取的成功结果，列表、404 与错误不缓存；miss 回源数据库并设置 TTL，成功更新/删除后失效。Key 包含 namespace、格式版本和主体/ID 摘要，不拼接原始主体；缓存结果校验主体与 ID，损坏或不匹配视为 miss。无数据库模式缓存问候计算结果，主体与 name 参与摘要，不增加持久化接口。

默认可选缓存：连接不可用或内部缓存预算用尽时记录脱敏事件，读取回源，不影响就绪；调用方取消/deadline 仍返回超时，不作为可选故障吞掉。required true：启动连接失败关闭客户端并失败，就绪依赖 Ping；读取和业务写入前检查失败返回 `unavailable`（HTTP 503）。缓存写入与 SQL 不具备跨系统事务，已完成的 SQL 写入不因后续缓存失效失败而报错；失效失败记录事件，旧值由 TTL 限制。并发 miss 与写入也可能回填旧值，首版不承诺强一致，资源撤销或强一致读取应绕过缓存。Redis 不能作为示例业务的唯一权威状态。

生命周期统一由 App 持有，启动失败及退出关闭连接池；可选依赖恢复后下一次操作重新使用连接。TLS、CA 或配置本身无效始终失败，不能借可选模式忽略安全配置错误。普通检查验证四种开启组合与默认裁剪；真实验收使用独立 Redis 实例、随机端口、密码、TLS 证书和 namespace，不 FLUSHDB/FLUSHALL，不修改已有实例，覆盖 TTL、命中/失效/回源、坏值、主体隔离、运行中取消、认证/TLS、必需依赖故障、恢复与关闭。

### Casbin 可选能力

`Options.Authz` 支持 `none`（默认）与 `casbin`，其他值在渲染前拒绝；可与四种存储选择及 Redis 组合，当前生成项目均使用 JWT。开启时才生成授权组件、配置、策略文档、测试和 Casbin 依赖。所有业务入口在 JWT 之后、业务处理之前执行授权；存活/就绪检查保持公共入口，未登记入口不获得权限。后续 HTTP/RPC 统一使用资源与操作，而不是将 URL 当作策略。

首版使用固定简单 RBAC 模型、精确匹配，不接受自定义模型、通配符、表达式、角色继承或超级管理员绕过。内置权限资源为 `example`，SQL 模式操作为 `read/create/update/delete`，无数据库模式仅 `read`。HTTP GET 列表和详情映射 read，POST 映射 create，PATCH 映射 update，DELETE 映射 delete。其他业务从模块的纯权限描述显式登记，未声明的资源或操作拒绝；声明权限不代表授予任何用户。Biz 始终继续校验主体及资源归属，拥有操作权限不代表可以访问他人的资源；JWT 主体不能自行指定角色。

模块通过 `module_<name>.go` 中的 `<name>Permissions()` 声明 HTTP method 与 RPC FullMethod 对应操作；工厂和登记使用同一来源，工厂返回不同映射时拒绝组装。生成索引的 `bootstrap.PermissionCatalog()` 汇总目录，启动与离线策略导入共用，不构造业务、不连接外部服务、不读取运行时 goseed.yaml。资源名沿用模块命名，操作是小写字母开头的小写字母/数字/下划线名称（最多 32 个字符）；拒绝非法 HTTP 方法、非法/重复 FullMethod 和未知权限。authz 及 SQL/File Store 显式接收并复制目录，后续修改调用方 map/slice 不改变校验边界。新组装入口版本为 2，旧入口或缺纯描述的工厂在生成前拒绝，需要人工显式升级。

策略 CSV 采用 `p, role:reader, example, read` 和 `g, user:alice, role:reader`；请求主体自动加 `user:` 前缀，角色只能为 `role:` 加小写字母开头的字母/数字/下划线名称（最多 32 个字符）。用户部分沿用 JWT 的 1 至 128 字符、UTF-8、无控制字符与首尾空白规则。仅接受 p 的三字段与 g 的两字段，不接受额外字段、未知资源/操作、重复规则或角色继承；所有用户与角色命名空间严格分离。策略最多 4096 条，文件最多 1MiB，文件必须为非符号链接的普通文件；未知/损坏策略及存储错误在启动时失败，空策略合法且默认拒绝。策略正文、主体和原始存储错误不进入日志。

SQL 策略复用已选 GORM 连接，独立 `casbin_rules` 表由 Goose 第 2 版迁移显式创建；字段为 id、ptype、v0、v1、v2，唯一约束保证规则不重复。主体列保持 UTF-8 二进制比较语义，字段非空与长度由迁移约束，启动核对迁移版本、必需列存在及每条已存策略的合法性。存储接口所有 Load/Replace 操作显式接收 Context；不使用 AutoMigrate 或 Casbin AutoSave，不在无 Context 的 SDK 回调中访问数据库。

生成服务提供 SQL 专用离线命令 `policy replace --file <csv> [--config file]`：先完整校验规则与候选权限模型，再在同一事务中替换策略；失败回滚已有规则，不只完成 DELETE。命令不自动迁移、无默认授予或角色创建。空文件显式导入表示撤销全部权限。运行实例使用启动加载的策略快照，离线修改后需重启全部实例；首版不提供跨实例同步。包内受控 Replace/Reload 使用完整候选快照，只有持久化或加载成功后才发布，失败保留旧快照；已进入授权判定的请求可能使用更新前快照。

无数据库模式只提供强类型 `authz.policy_file`，默认 `configs/policy.csv`，可由 `APP_AUTHZ_POLICY_FILE` 覆盖；生成文件只有说明注释，不默认授予 alice 或其他主体权限。启动只读加载，缺失/非法文件失败，不提供策略写入命令或接口；更新文件后需重启。SQL 模式不接受 policy_file 配置，CSV 文件仅用于显式离线导入。

授权判定使用普通同步 Enforcer，关闭判定缓存和 g 函数缓存；只发布经过校验的完整模型，不暴露 SDK 指针。调用方 Context 在加载、保存和判定前后检查；未知资源/操作、无角色或无权限均拒绝，未认证返回 401、权限不足返回 403，内部模型错误返回脱敏 500，调用方取消/deadline 返回 504。

验收覆盖开启/关闭时的文件、依赖、配置与迁移裁剪，四种存储及 Redis 的 16 种渲染组合；所有新增开启组合完成 readonly 测试/vet/race/lint 与 Linux 纯 Go 构建。真实 SQLite、MySQL、PostgreSQL 和只读文件验证默认拒绝、允许/拒绝、角色命名碰撞、主体隔离、导入与撤销、重载、输入边界、取消、存储失败及事务回滚；Redis 组合验证缓存命中不会绕过授权。外部数据库验收显式运行且不得跳过，不触及已有数据库或生产策略。

## 纯 gRPC 模板

`Options.Transport` 支持 `http`（默认）、`grpc`、`dual` 与 `gateway`；dual 约束见 [dual 模板](templates.md#dual-模板)，Gateway 约束见 [Gateway 模板](templates.md#gateway-模板)。复用 Biz、Store、JWT、Casbin 与 Redis，纯 gRPC 不交付 Gin、HTTP 配置／中间件、Swagger 或业务 HTTP 服务。

Proto 源位于 `api/proto/example/v1/`，已生成代码位于 `pkg/api/example/v1/`，供其他 Go 服务导入协议类型；Handler 位于 `internal/apiserver/handler/grpc/example/`，服务及拦截器位于 `internal/pkg/rpc/`。提供示例客户端和固定版本的显式协议生成入口，普通构建不运行 protoc、不安装工具或 tidy。四种存储 × Redis 开关 × Casbin 开关共 16 种组合均需 readonly 编译，代表组合完成实际调用。

资源模块各自使用 `api/proto/<module>/v1/<module>.proto` 与 `pkg/api/<module>/v1/`。`scripts/proto.py` 按模块路径排序并复现所有模块协议，检查 protoc 与插件固定版本；新资源的 HTTP、RPC 与 dual 渲染共用一次 Store/Biz 组装，dual 的两种 Handler 接收同一个 Biz。资源 CRUD 使用独立 `<Name>Service`，权限目录按每个服务的 FullMethod 精确映射；创建必需字段与 PATCH 更新字段采用 Proto3 optional 保留出现性，不暴露 owner。

SQL 提供 CRUD，无数据库提供认证问候。Proto3 optional 保留 PATCH 字段存在性和零值；分页省略取 1/20，显式零由 Biz 拒绝；ID 限 1 至 int64 最大值。拒绝未知 Proto 字段，文本、分页和资源归属由同一 Biz 校验。

### 配置与安全

强类型 grpc 配置：address、request_timeout、shutdown_timeout、max_recv_bytes、max_send_bytes、max_header_bytes、max_concurrent_streams、tls_cert、tls_key、reflection。默认回环 `127.0.0.1:9090`、请求 5s、关闭 10s、接收 1MiB、发送 2MiB、Header 64KiB、每连接并发 128、Reflection 关闭。环境绑定 `APP_GRPC_*`，`serve --address` 覆盖当前协议。时长为带单位字符串，限 1ms 至 10m；消息上限 1 至 16MiB，Header 1KiB 至 4MiB，并发 1 至 1024。证书／私钥成对，TLS 最低 1.2；生产要求 TLS，开发可明文。Reflection 仅开发回环可开启，并要求 JWT。

业务 unary 要求单个 `authorization: Bearer <token>`，JWT 主体进入 Context。Casbin 在缓存／Biz 前按精确 FullMethod 映射 example 的 read/create/update/delete，无数据库只有 read。未知方法不获授权。请求 ID 采用单个 `x-request-id`，仅接受 1 至 64 个 ASCII 字母、数字、点号、下划线或连字符；省略时随机生成，响应回传，日志与 Store 共用 Context。日志仅输出已登记方法、请求 ID、Status 与耗时，不输出 Token、metadata、正文、主体或原始 cause；panic 脱敏为 Internal。

保留调用方更短的 deadline 与取消。错误映射为 InvalidArgument、Unauthenticated、PermissionDenied、NotFound、AlreadyExists、Unavailable、Canceled、DeadlineExceeded 和 Internal，内部原因不透传；消息大小由 transport 限制。

### 健康与退出

标准 Health Check／Watch 为显式公共入口。空服务名和业务服务名表示依赖就绪，`liveness` 表示存活；未知服务名 NotFound。Check 当次校验依赖，Watch 最多每 500ms 更新，依赖预算 200ms；可选 Redis 故障不阻断，必需 Redis／数据库故障为 NOT_SERVING，恢复后 SERVING。关闭先停止就绪、通知 Watch，再 GracefulStop；超时强制 Stop 并取消请求，等 Serve 返回后关闭池。

不生成业务流。Stream 拦截器仅放行 Health Watch，以及已显式开启且通过 JWT 的 Reflection；其他流式入口拒绝。证书或监听失败关闭已初始化资源，不自动迁移。

### 验收与依据

验证 16 组合裁剪、固定清单、Linux 纯 Go 构建；代表产物执行 test/vet/race/lint 与真实 RPC，覆盖 CRUD／问候、零值／分页、归属与角色隔离、未知字段、JWT/Casbin、请求 ID、取消/deadline、panic／原始错误脱敏、消息限制、健康 Watch／故障恢复、优雅与强制退出、TLS 信任／主机名、Reflection、启动失败、持久化及日志脱敏。实际执行交付的 Proto 生成脚本逐字节比对产物，并用独立 Go 模块导入 `pkg/api/example/v1`；新增 RPC 的权限登记遵循安全链要求。真实 SQL／Redis 沿用隔离夹具，禁止跳过或触及已有实例；生产 TLS 容器验证非 root、只读根文件系统与证书／策略只读挂载。

依据：[Health 文档](https://grpc.io/docs/guides/health-checking/) 、[优雅关闭文档](https://grpc.io/docs/guides/server-graceful-stop/) 、[Reflection 文档](https://grpc.io/docs/guides/reflection/) 。固定版本及维护证据沿用本日已核验的 [依赖方案](dependencies.md) 。

## dual 模板

`Options.Transport: "dual"` 选择双协议模板。Gin HTTP 与 gRPC 使用独立 Handler，共用同一个 Biz 实例、JWT、所选存储及可选 Redis 和 Casbin；启用数据库时共用 Store 与连接池，无数据库时仅采用 Handler → Biz。HTTP 不转发到 gRPC。当前交付与后续检查点状态统一见 [ROADMAP](../ROADMAP.md)。

### 配置与源码

同时保留 HTTP、Swagger 2.0、Proto、公开 `pkg/api`、gRPC 示例客户端及固定版本显式协议生成入口。HTTP 与 gRPC 分别使用现有协议配置与安全链，生产 gRPC 必须 TLS，Reflection 只允许开发回环并认证；HTTP 的 TLS、CORS、pprof 规则保持不变。

默认 HTTP 地址 `127.0.0.1:8080`，gRPC 地址 `127.0.0.1:9090`。`serve --address` 只覆盖 HTTP，`serve --grpc-address` 只覆盖 gRPC；环境变量分别为 `APP_HTTP_ADDRESS` 与 `APP_GRPC_ADDRESS`。两个显式监听地址相同且端口非零时配置校验拒绝；随机端口零可用于隔离测试。开发、迁移及策略命令不接受监听地址覆盖。

Redis 命令预算必须小于两个协议的请求预算。所有业务入口按 JWT → Casbin → Biz／缓存执行，资源归属与输入规则由共享 Biz 保证；协议特有错误结构和字段类型保持各自契约，不要求 HTTP JSON 与 Proto JSON 逐字节相同。

### 生命周期

启动先绑定两个监听器，任何监听失败都关闭本次已绑定监听器，不启动单独可用的半个服务。证书或依赖初始化失败关闭已创建资源，服务启动不自动迁移。

两个服务并发运行；收到退出信号或任一服务返回时，先停止两侧就绪，再取消两侧服务生命周期。HTTP 与 gRPC 并发排空，各使用其配置的关闭预算，整体排空预算为二者较大值，而非两者相加；超时分别强制关闭连接／RPC。两个 Serve 均返回后再统一关闭缓存、数据库和日志。健康 Watch 和开发 Reflection 遵循纯 gRPC 规则。

### 验收

四种存储 × Redis 开关 × Casbin 开关共 16 种 dual 组合均需生成、readonly 编译及 Linux 纯 Go 构建。SQLite／无数据库代表组合完成 test、vet、race、lint、实际 HTTP／RPC 调用与 Proto 复现。

覆盖跨协议创建、读取、零值更新、删除、分页／文本边界、JWT 失败、主体与角色隔离、权限撤销先于缓存、请求 ID、取消与 deadline；验证两个端口分别占用、证书失败、启动清理、依赖关闭、同时排空、强制关闭、任一 Serve 异常和重启持久化。真实 MySQL／PostgreSQL、TLS Redis 与生产双端口容器使用现有隔离夹具，不触及已有实例。

现有协议契约见 [HTTP](templates.md#默认-http-模板) 与 [gRPC](templates.md#纯-grpc-模板) ，缓存和授权见 [数据库与可选能力](templates.md#数据库与可选能力) 。

## Gateway 模板

`Options.Transport: "gateway"` 选择 HTTP → gRPC Handler → Biz 的真实转发模式；保留直接 gRPC 入口，不生成 Gin、HTTP 业务 Handler 或重复 Biz。四种存储与 Redis／Casbin 开关组合，共 16 种；当前交付状态见 [ROADMAP](../ROADMAP.md)。

### 协议与输入

SQL 使用 `/v1/examples` 的 POST／GET，以及 `/v1/examples/{id}` 的 GET／PATCH／DELETE；无数据库仅 GET `/v1/examples?name=...`。映射放在 Proto 同目录的固定 HTTP 配置文件，Swagger 安全声明位于同目录的 `openapi.yaml`；两份配置由固定版本插件读取，普通构建使用已生成 Go 和 Swagger 2.0。显式生成不修改业务文件或依赖清单。

成功响应采用 ProtoJSON，字段使用 Proto 的 snake_case，64 位整数为字符串；所有成功方法返回 200，Delete 返回 `{}`。PATCH 保留 optional 的零值语义。请求拒绝未知／重复／null 字段、额外 JSON、非法 UTF-8、非法／重复／未知 query、正文超限及正文中的路径 ID；路径 ID 只取 URL。POST／PATCH 只接受 JSON。不启用 HTTP 方法覆盖或表单回退。错误采用 Gateway 的数字 gRPC code、脱敏 message 与空 details，HTTP 状态按标准 gRPC 映射；请求 ID 在响应头提供。

只转发 Authorization 与合法 X-Request-ID，拒绝重复认证头，忽略客户端伪造的 Grpc-Metadata-* 身份。JWT → Casbin → Biz／缓存在 RPC 入口执行；HTTP Context、取消和预算传入 RPC，不接受客户端无限延长预算。访问日志只记方法、固定路由、状态、请求 ID 与耗时，不含 Token、正文、query 或动态路径。

CRUD 资源通过独立 RPC Handler 和 `http.yaml` 映射接入，不生成重复 Gin Handler；模块工厂登记固定路由、允许的 query 和请求方法。`make proto-generate` 发现 `api/proto/<module>/v1/*.proto` 下的全部模块，使用锁定工具在临时目录生成 Go、Gateway 和 Swagger，再由 `scripts/openapi` 严格合并模块文档并更新登记摘要。任一生成或合并失败时不更新输出；完成后复现应逐字节一致。Gateway Swagger 源由 Proto 与映射配置生成，不作为手工编辑文件。

### 配置与生命周期

HTTP 默认 `127.0.0.1:8080`，gRPC 默认 `127.0.0.1:9090`；`serve --address` 与 `--grpc-address` 分别覆盖。沿用两侧预算、TLS、CORS、健康和开发 pprof 约定。新增 `gateway.ca_file`、`gateway.server_name` 仅用于本进程 Gateway 到本进程 RPC 的证书验证；生产 gRPC 必须 TLS，绝不跳过证书校验。客户端目标由实际监听地址确定，通配监听转为回环，不提供任意上游地址或独立外部服务模式。

先绑定两端口，启动 RPC，再在有限启动预算内完成健康 RPC 和 TLS 验证，最后开放 HTTP。任一启动失败回收监听器、RPC、转发连接和共享依赖；不自动迁移。正常退出先停止两侧就绪，排空 HTTP，HTTP 返回后才关闭转发连接、排空 RPC，最后关闭共享依赖。关闭总预算为两侧预算之和。RPC 意外返回时同样回收 HTTP，允许其在预算内失败退出。

### 验收

16 种组合 readonly 编译、Linux 纯 Go 构建；SQLite／无数据库代表项目 test、vet、race、lint、实际 HTTP／RPC、Proto／Gateway／Swagger 逐字节复现。验证认证授权先于缓存、跨协议 CRUD／问候、零值更新、输入边界、错误脱敏、请求 ID、预算取消、端口占用、证书失败、失败回收和顺序排空。真实 MySQL／PostgreSQL、TLS Redis 和双端口生产容器复用隔离夹具，无外部用例跳过。

业务规则及协议边界见 [gRPC](templates.md#纯-grpc-模板) 与 [数据库与可选能力](templates.md#数据库与可选能力) 。实现采用 [官方外部 HTTP 映射方式](https://grpc-ecosystem.github.io/grpc-gateway/docs/mapping/grpc_api_configuration/) 与 [Gateway 定制接口](https://grpc-ecosystem.github.io/grpc-gateway/docs/mapping/customizing_your_gateway/) 。
