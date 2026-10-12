# goseed

面向中小型 Go 后端业务的项目脚手架与业务代码生成器。生成结果拥有完整源码，可以独立开发、测试和部署，不需要 goseed 运行时框架。

默认在空目录生成 **HTTP + SQLite + JWT** 项目；通过参数选择数据库、协议、Redis 和 Casbin。持久化业务采用 Handler → Biz → Store 分层，配置、日志、认证、迁移和服务生命周期按清晰边界组织。

## 当前能力

| 能力 | 当前交付 |
|---|---|
| 项目生成 | 无参数默认项目、项目命名、Go module、dry-run、冲突保护 |
| 数据库 | SQLite、MySQL、PostgreSQL、无数据库；按选项裁剪驱动、配置和源码 |
| 协议 | HTTP、gRPC、dual、Gateway；已有示例业务与对应安全链 |
| 认证 | JWT 为固定基础能力，提供明确的本地开发签发工具 |
| 授权 | 可选 Casbin RBAC、默认拒绝、SQL 或只读文件策略 |
| 缓存 | 可选 Redis；示例缓存、TTL、可选回源与必需依赖检查 |
| 工程基础 | 强类型配置、Zap 日志、Context、业务错误、显式迁移、健康检查、TLS 与有限期关闭 |
| 部署文件 | 独立 Go module、Makefile、非 root Dockerfile、所选能力的开发配置 |
| 业务模块 | `gen module` 创建无业务操作的结构骨架，登记协议与文档 |
| 资源内部渲染 | CRUD 核心、SQL 草稿、HTTP Handler/工厂/Swagger 源；内部只读接口 |

四种协议 × 四种存储 × Redis 开关 × Casbin 开关，共 64 种初始项目组合。生成结果和必要运行依赖按能力选择，不默认引入所有驱动与服务。当前命令契约见 [CLI](docs/cli.md)，验收与进度见 [ROADMAP](ROADMAP.md)。

## 从源码构建

Go 固定为 **1.26.4**；主模块为 `github.com/setcreed/goseed`。需要 Git、Make 和 Python 3.9+；首次构建需要下载固定 Go 工具链及依赖。

```sh
make contract-check
make build
./bin/goseed --help
./bin/goseed --version
```

使用刚构建的可执行文件生成独立项目：

```sh
GOSEED_BIN="$(pwd)/bin/goseed"
mkdir ../shop
cd ../shop

# 项目名和 module 独立指定；先查看计划，再写入
"$GOSEED_BIN" --name shop --module github.com/acme/shop --dry-run
"$GOSEED_BIN" --name shop --module github.com/acme/shop
```

下面是另一种能力组合，应在另一个空项目目录执行：

```sh
goseed --name shop --module github.com/acme/shop \
  --db=postgres --redis --authz=casbin --transport=dual
```

上例假定已将构建的 goseed 加入 PATH。生成命令只生成文件，不执行 Git、依赖安装、数据库迁移、容器启动或部署。已有文件冲突会被拒绝；业务项目的运行步骤以它自身生成的 README 为准。

## 为已有项目增加模块

在 goseed 生成的业务项目目录执行：

```sh
goseed gen module orders --dry-run
goseed gen module orders
```

该命令创建 Biz/Handler 与所选数据库的 Store 骨架，生成一次性模块工厂，安全更新元数据、协议登记和 OpenAPI 汇总。Handler 不虚构业务端点，开发者补齐领域规则、路由与权限。既有手工工厂和业务源码受到保护，写入失败恢复本次新增/更新内容。

资源内部入口 `generator.PrepareResource` 目前提供只读计划、CoreFiles、HTTPFiles、SchemaStore 与可选 MigrationDraft；查询返回独立副本，不连接数据库或写入文件。各接口用途与兼容边界见 [资源接入方案](docs/resource-generation.md)。

## 开发与验证

```sh
# 项目内固定工具；不安装全局依赖
make tools
make protocol-tools

# 全仓检查，含独立生成项目和回环进程验收
make check
make race

# 显式真实服务/容器验收，需要 Docker、Compose 和 OpenSSL
make template-db
make template-redis
make template-container
```

普通检查不启动 Docker；外部数据库/Redis 用例缺少专用凭据时可能跳过，显式真实服务目标禁止跳过。所有服务验证使用本次隔离资源、随机回环端口和独立数据，结束仅清理本次对象。验证临时项目与缓存位于 `.tmp/`，工具位于 `.tools/`，均不入 Git。

换电脑恢复、分层源码导航和定向检查命令见 [开发指南](docs/development.md)。项目固定版本见 [dependencies.lock.json](dependencies.lock.json)，维护与许可证核验记录见 [依赖方案](docs/dependencies.md)。历史验证结果不能代替新环境重跑。

## 文档入口

| 文档 | 内容 |
|---|---|
| [AGENTS](AGENTS.md) | 项目规范、工程边界和协作流程 |
| [需求](docs/requirements.md) | 产品场景、默认行为、首版范围与验收 |
| [架构](docs/architecture.md) | 目录、分层、配置、日志、存储和生命周期 |
| [CLI](docs/cli.md) | 参数、生成行为、字段规范与文件归属 |
| [施工清单](docs/implementation.md) | 从零建设的阶段、依赖、失败路径和交付门槛 |
| [开发指南](docs/development.md) | 新电脑恢复、实现入口、验证与公开准备 |
| [资源接入](docs/resource-generation.md) | 业务生成的计划、协议、权限与兼容设计 |
| [HTTP](docs/template-http.md) | 默认服务运行与输入契约 |
| [可选能力](docs/template-options.md) | SQL、无数据库、Redis 与 Casbin |
| [gRPC](docs/template-grpc.md)、[dual](docs/template-dual.md)、[Gateway](docs/template-gateway.md) | 协议、安全链和排空约定 |

## 许可证

[MIT](LICENSE)。第三方依赖及生成代码中的版权/许可说明继续适用，详细范围见 [许可证记录](docs/dependencies.md#许可证范围)。
