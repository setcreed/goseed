# 可选模板契约

本文件定义 D3 的模板组合与验收约定。当前交付状态以 ROADMAP 为准；生成命令的写入入口仍在 D5 接入。

## SQL 数据库变体

内部 `templates.Render(Project, Options)` 返回完整文件计划；`Options.Database` 为空时采用 SQLite，数据库枚举为 `sqlite`、`mysql`、`postgres`、`none`。未知枚举直接拒绝，不生成部分文件。`Default` 保持默认模板语义。

选择发生在生成阶段：产物只包含对应驱动、配置字段、SQL 方言、测试与使用文档，不在运行时切换数据库。共用 Handler → Biz → Store、JWT、错误和生命周期。依赖清单按变体固定，构建使用 readonly；渲染不联网、不 tidy、不写文件。

SQLite 保持文件、连接 PRAGMA 和单实例约定。MySQL、PostgreSQL 采用强类型 host/port/user/password/name，不接受自由 DSN；默认端口分别为 3306、5432，用户及数据库名默认项目名，密码无默认值。连接池默认最多 4 个连接、2 个空闲连接，连接寿命 30m、空闲寿命 5m、连接预算 5s，慢查询阈值 200ms。连接预算限制在 1s 至 30s，池寿命必须为正且空闲寿命不得超过连接寿命。

外部数据库默认 `database.tls: true`，TLS 至少 1.2，校验服务端证书及 host；可通过 `database.ca_file` 增加私有 CA。禁止跳过验证及 TLS 回退。只有 development 允许 tls 为 false；关闭 TLS 时不能设置 CA。错误与日志不输出密码、连接串、SQL 和参数。MySQL 使用独立 Connector，PostgreSQL 使用独立 pgx 配置，共用现有连接池，不注册全局 TLS 配置。

JWT 主体限制为 1 至 128 个 Unicode 字符，拒绝控制字符及首尾空白，三个数据库保持一致的资源归属与名称唯一语义。MySQL 使用 utf8mb4 二进制排序规则；PostgreSQL 文本使用 C 排序规则。所有读写时间使用 UTC。MySQL 更新使用匹配行数，重复写入相同值不能把已有资源误报为 404。

显式 `migrate status/up` 共用命令；status 和启动检查不创建 Goose 表，服务启动只核对版本及实际列，就绪依赖 Ping。SQLite/PostgreSQL 迁移使用方言支持的事务；MySQL DDL 不具备事务回滚，迁移失败后必须核对实际 schema，不通过自动重试修补部分 schema。退出和启动失败均关闭连接池。

外部数据库的 `dev init` 只创建开发 JWT 密钥并设置开发明文连接；需要先由环境安全提供数据库密码，再执行保留已有配置、Token、迁移与服务命令。脚手架不部署数据库、不创建生产账号。

## 验收

普通检查对三个 SQL 变体执行实际渲染、readonly 测试、vet、race、lint 与构建，并核对驱动裁剪、清单不可变及文档链接。外部数据库未配置时，生成项目明确跳过需要真实服务的测试，不能据此声明数据库已验证。

`make template-db` 显式启动本次隔离的固定摘要 MySQL、PostgreSQL 容器、全新数据卷和随机回环端口。测试账号仅用于该次验证；测试夹具创建随机命名的独立数据库，结束关闭连接并仅删除自己创建的数据库。真实验收禁止跳过外部用例，覆盖空库只读检查、迁移重放、约束与大小写、事务回滚、零值更新、Context、数据库关闭、错误脱敏、HTTP 跨主体隔离与服务重启持久化。

数据库认证失败、TLS 不可信、无效 CA、取消、缺 schema、错误 schema 或池关闭均必须失败；启动失败不能泄露连接池。基础 CRUD 验证不代替业务上线前的容量、备份恢复和数据库权限评估。

## 无数据库变体

`Database: "none"` 在生成时移除数据库配置、ORM、数据库驱动、迁移、模型与 Store；目录 `variants/none/` 仅维护无数据库示例、必要测试、依赖清单和使用文档的替换模板，其余 HTTP/JWT 基础能力共用基础模板。生成项目不使用内存仓储模拟持久化。

示例采用 Handler → Biz，`GET /v1/examples` 必须通过 JWT 认证，返回 `{"message":"hello, world","subject":"alice"}`。可选 query `name` 默认 `world`，显式提供时为 1 至 80 个 Unicode 字符，拒绝无效 UTF-8、控制字符与首尾空白。Handler 拒绝未知或重复参数及错误百分号编码；Biz 校验输入、请求主体并传播取消或 deadline。没有资源 CRUD 路由、写入或重启持久化承诺。

保留配置优先级与强类型校验、开发密钥初始化/签发、HTTP TLS/CORS、日志脱敏、请求预算和优雅退出；就绪只反映服务生命周期。`migrate` 命令直接报错且不创建文件，帮助与 Makefile 不登记迁移；`make dev` 仅初始化并启动。旧数据库 YAML 字段作为未知配置拒绝。Docker 继续使用固定构建镜像与非 root scratch 运行，不创建或声明数据库数据目录。

验收以实际生成模块为对象：readonly 测试、vet、race、lint、本机构建与 Linux 纯 Go 构建；核对依赖树和交付文件不含 ORM/驱动/迁移，清单不变。真实进程无需数据库即可启动，覆盖 JWT、两个主体、输入边界、未登记方法/资源、重启、退出及日志脱敏。数据库模板继续执行原有回归。

## Redis 可选能力

`Options.Redis` 默认 false，可与四个数据库枚举组合。开启时才生成 `internal/pkg/cache/`、Biz 缓存接口与示例、配置、依赖、测试、Swagger 错误响应和 `docs/redis.md`。`variants/redis/common/` 保存共用客户端及缓存模板；各数据库子目录保存对应完整依赖清单。运行时不提供启停功能开关。缓存不改变 JWT 或 Casbin 存储，不包含集群、锁、队列或限流。

强类型配置为 address、username、password、db、key_prefix、ttl、command_timeout、dial_timeout、pool_size、required、tls、ca_file。默认地址 `127.0.0.1:6379`、DB 0、项目名 namespace、TTL 30s、命令预算 200ms、拨号上限 1s、池大小 4、required false、TLS true；无默认密码。TTL 限 1s 至 5m，命令预算限 1ms 至 5s 且小于 HTTP 请求预算，拨号限 1ms 至 5s，池大小 1 至 64，DB 0 至 15，namespace 只允许 1 至 64 个 ASCII 字母/数字/冒号/下划线/连字符。生产需要密码和验证证书的 TLS 1.2+，私有 CA 通过 ca_file 加入；开发初始化设置开发明文连接，不提供密码或部署 Redis。真实密钥和凭据不进入日志。

每个操作保留调用方 Context，另加短命令 deadline；明确读写/池等待上限、关闭自动重试与客户端缓存/实验流水线。提前取消立即拒绝，运行中纯取消最多等待短命令预算后返回调用方取消；不声称修复客户端上游的无 deadline 取消问题。

SQL 示例只缓存按主体与 ID 读取的成功结果，列表、404 与错误不缓存；miss 回源数据库并设置 TTL，成功更新/删除后失效。Key 包含 namespace、格式版本和主体/ID 摘要，不拼接原始主体；缓存结果校验主体与 ID，损坏或不匹配视为 miss。无数据库模式缓存问候计算结果，主体与 name 参与摘要，不增加持久化接口。

默认可选缓存：连接不可用或内部缓存预算用尽时记录脱敏事件，读取回源，不影响就绪；调用方取消/deadline 仍返回超时，不作为可选故障吞掉。required true：启动连接失败关闭客户端并失败，就绪依赖 Ping；读取和业务写入前检查失败返回 `unavailable`（HTTP 503）。缓存写入与 SQL 不具备跨系统事务，已完成的 SQL 写入不因后续缓存失效失败而报错；失效失败记录事件，旧值由 TTL 限制。并发 miss 与写入也可能回填旧值，首版不承诺强一致，资源撤销或强一致读取应绕过缓存。Redis 不能作为示例业务的唯一权威状态。

生命周期统一由 App 持有，启动失败及退出关闭连接池；可选依赖恢复后下一次操作重新使用连接。TLS、CA 或配置本身无效始终失败，不能借可选模式忽略安全配置错误。普通检查验证四种开启组合与默认裁剪；真实验收使用独立 Redis 实例、随机端口、密码、TLS 证书和 namespace，不 FLUSHDB/FLUSHALL，不修改已有实例，覆盖 TTL、命中/失效/回源、坏值、主体隔离、运行中取消、认证/TLS、必需依赖故障、恢复与关闭。

## 后续能力

Casbin 依照架构与命令契约实施，具体配置和失败行为在对应功能实施前补充本文件。
