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
| 可观测性 | 可选 OpenTelemetry 指标与追踪，OTLP/HTTP 主动推送；默认关闭 |
| 单机部署 | 显式生成 systemd/Nginx 配置副本、权限/TLS 指引与所选数据库备份恢复脚本 |
| 工程基础 | 强类型配置、Zap 日志、Context、业务错误、显式迁移、健康检查、TLS 与有限期关闭 |
| 部署文件 | 独立 Go module、Makefile、非 root Dockerfile、所选能力的开发配置 |
| 业务模块 | `gen module` 创建无业务操作的结构骨架，登记协议与文档 |
| CRUD 资源 | `gen resource` 按 spec 生成主体隔离 CRUD、四协议适配、登记与 OpenAPI 汇总，可选 SQL 迁移草稿 |

四种协议 × 四种存储 × Redis 开关 × Casbin 开关，共 64 种初始项目组合。生成结果和必要运行依赖按能力选择，不默认引入所有驱动与服务。当前命令契约见 [CLI](docs/cli.md)，验收与进度见 [ROADMAP](ROADMAP.md)。

## 从源码构建

最低要求为 Go **1.26.4**；日常构建使用本机已安装的兼容版本，低于最低版本时会报错，不自动下载工具链。主模块为 `github.com/setcreed/goseed`。需要 Git、Make 和 Python 3.9+；首次构建仍需下载项目依赖。

```sh
git clone https://github.com/setcreed/goseed.git
cd goseed
make contract-check
make build
./bin/goseed --help
./bin/goseed --version
```

从固定源码版本安装到指定目录：

```sh
git checkout v0.1.0
make install VERSION=v0.1.0 GOBIN="$HOME/.local/bin"
"$HOME/.local/bin/goseed" --version
```

`v0.1.0` 仅为命令示例；执行前应替换成仓库中已存在且经过审核的 tag。本项目尚未创建 tag 或公开 release。

本地生成单平台归档并校验：

```sh
make package VERSION=v0.1.0 GOOS=darwin GOARCH=arm64
make verify-package ARCHIVE=.tmp/dist/v0.1.0/goseed_v0.1.0_darwin_arm64.tar.gz
```

归档包含生成器、README、MIT 许可证、构建依赖许可证副本和版本清单；清单记录源提交、生成器/模板/spec 版本及二进制摘要。SHA-256 文件用于检测归档损坏，不是签名或来源证明。`make install` 需要显式指定 `GOBIN` 和版本；为保证归档可复现，`make package` 固定使用 Go 1.26.4，并拒绝未固定版本或有改动的工作区。本地归档保存在忽略目录 `.tmp/dist/`，当前不代表公开发行包。

`goseed --version` 同时显示生成器版本、源码提交、模板版本和资源 spec 版本；项目元数据与旧项目兼容边界、人工升级方式见 [开发指南](docs/development.md#版本兼容与人工升级)。

本地二进制运行证据：

| 目标 | 归档构建 | 运行与生成检查 |
|---|---|---|
| macOS arm64 | 通过 | 本机运行通过 |
| macOS amd64 | 通过 | Apple Silicon 上经 Rosetta 2 运行通过；未在 Intel Mac 实机验证 |
| Linux arm64 | 通过 | 固定 Alpine 构建容器内运行与项目生成通过 |
| Linux amd64 | 通过 | Debian amd64 容器模拟运行、默认项目生成及 HTTP 健康/认证创建与列表检查通过；未在原生 amd64 硬件验证 |

Windows 原生运行不在当前支持范围。交叉构建成功本身不代表目标平台已验证；后续实际平台证据以路线图为准。

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

`--metrics` 与 `--tracing` 可独立选择；它们默认关闭，启用时需在生成项目的运行环境设置独立 OTLP/HTTP 信号出口，见生成项目 README。

`--deployment=single-host` 显式生成 systemd/Nginx 单机部署副本，默认不增加这些文件；SQLite、MySQL 和 PostgreSQL 项目会生成对应备份工具说明。样例不会安装系统服务、写主机目录或运行迁移。实际 systemd/Nginx 隔离运行验收状态见 [ROADMAP](ROADMAP.md)。

## 为已有项目增加模块

在 goseed 生成的业务项目目录执行：

```sh
goseed gen module orders --dry-run
goseed gen module orders
```

该命令创建 Biz/Handler 与所选数据库的 Store 骨架，生成一次性模块工厂，安全更新元数据、协议登记和 OpenAPI 汇总。Handler 不虚构业务端点，开发者补齐领域规则、路由与权限。既有手工工厂和业务源码受到保护，写入失败恢复本次新增/更新内容。

为业务项目增加 CRUD 资源：

```sh
goseed gen resource product --spec specs/product.yaml --dry-run
goseed gen resource product --spec specs/product.yaml --migration
```

`--spec` 必须位于项目 `specs/` 目录。gRPC、dual 和 Gateway 项目需要先在业务项目内执行 `make proto-tools`，生成器只使用项目锁定的本地工具。`--migration` 只创建待审核 SQL 草稿，不连接数据库或执行迁移；使用字段、写入保护与失败回滚契约见 [CLI](docs/cli.md) 和 [资源接入方案](docs/resource-generation.md)。

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
| [模板指南](docs/templates.md) | HTTP、数据库与可选能力、gRPC、dual、Gateway 的运行和验收契约 |

## 许可证

[MIT](LICENSE)。第三方依赖及生成代码中的版权/许可说明继续适用，详细范围见 [许可证记录](docs/dependencies.md#许可证范围)。
