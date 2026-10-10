# 项目进度

## 当前状态

文档基线与 D1 技术核验、工程入口已完成必要验证，首次基线提交为 `01c8d6c`，本地分支为 `main`，未配置远端。主模块为 `github.com/setcreed/goseed`，工具链固定为 Go 1.26.4；下一检查点是 D2，业务模板尚未开始。

当前授权覆盖的 D1 已完成：依赖与工具核验、版本锁定、最小 CLI 和工程检查入口。需求以 [产品需求](docs/requirements.md) 为准，实施顺序见 [实施方案](docs/implementation.md) 。

## 进行中

暂无。D1 交付与验证记录见下方；后续检查点按用户指令推进。

## 待开始

| 检查点 | 交付 | 依赖 |
|---|---|---|
| D2 | HTTP + SQLite + JWT 默认模板 | D1 |
| D3 | MySQL、PostgreSQL、无数据库、Redis、Casbin | D2 |
| D4 | gRPC、dual、Gateway | D2；相关组合依赖 D3 |
| D5 | 全参数项目生成器与 dry-run | D2、D3、D4 |
| D6 | 模块与 CRUD 业务生成器 | D5 |
| D7 | 首版覆盖核验和使用文档 | D6 |

所有检查点均需完成对应验收后才能标记完成。

## 待核验与待确认

- 待核验：D3 的正式 Casbin 策略适配实现、输入约束与请求 Context；D1 原型已在三种数据库通过，不作为业务实现复用。
- 待核验：D3 的真实 Redis 行为、故障与生命周期；D1 仅完成客户端构造和版本兼容验证。
- 待确认：goseed 自身许可证；仅在公开发布前必须确定。主模块已确定为 `github.com/setcreed/goseed`，Go 工具链固定为 1.26.4。
- 待细化：CRUD 字段声明完整格式、生成登记接口、开发初始化和测试 Token 命令；对应检查点先补充契约再实现。

## 最近完成

- 2026-10-11 00:16 文档同步：明确目标默认行为与当前 CLI 能力的文档入口；README 补充真实数据库及协议验证程序范围、纯文档检查命令；实施方案改为引用 ROADMAP 状态，补记首次提交并合并重复验证记录。
- 2026-10-11 00:12 Git 工作流：用户授权每个功能完成并通过验证后提交，规则已写入 AGENTS；当前 D0、D1 交付纳入首次基线提交，后续以可独立验收的功能为提交单位。
- 2026-10-11 00:08 D1：确定 Go 1.26.4 与 `github.com/setcreed/goseed`；交付最小 CLI、Make 检查入口、独立兼容性模块、依赖锁及官方维护证据、项目内协议工具、固定摘要的隔离数据库验证。SQLite 驱动选 libtnb，策略存储采用复用现有连接的最小适配方案，文档输出固定 Swagger 2.0。CLI 当前仅支持帮助和版本，未实施业务生成。交付：[README](README.md) 与 [依赖核验](docs/dependencies.md) 。
- 2026-10-10 23:16 D0：初始化 `main` 分支，建立项目规范、需求、架构、命令契约、实施方案和文档导航；默认 SQLite/JWT，三种数据库、Redis、Casbin 与四种协议进入首版范围。交付入口：[README](README.md) 与 [实施方案](docs/implementation.md) 。

## 最近验证

- 2026-10-11 00:16 文档更新：`make contract-check` 通过模块/工具契约、20 个本地文档链接、Python 语法与 34 个交付文件格式；`git diff --check` 通过。README 验证范围逐项核对测试源码及 D1 记录，变更仅涉及 4 份文档，沿用已有运行证据。
- 2026-10-11 00:13 功能提交工作流与基线：`make check` 通过模块/工具契约、19 个本地文档链接、34 个交付文件、格式、CLI 测试、vet 和 lint；核对提交范围仅包含项目交付文件，`.tmp/`、`.tools/` 和 `bin/` 保持忽略。数据库与协议实现未变化，沿用 D1 已通过的运行证据。
- 2026-10-11 00:08 D1 工程与协议：`make tools`、`make protocol-tools` 成功，仅安装到项目 `.tools/`；linter 实际由 Go 1.26.4 编译。`make check`、`make race build`、`make protocol-check` 与独立模块 lint/race 通过；二进制元数据确认工具链与主模块，CLI 进程退出码及无写入行为通过。`make compatibility-cross` 对运行依赖及生成协议的两个测试包在 Linux amd64/arm64、Windows amd64 完成纯 Go 编译；实际 gRPC/Gateway 回环与 Swagger 2.0 解析通过。
- 2026-10-11 00:08 D1 真实数据库：`make compatibility-db` 使用固定 RepoDigest 的 MySQL 8.4.11、PostgreSQL 18.6，在全新隔离数据库完成显式迁移、缺表失败且不建表、默认拒绝、角色权限、重载、删除与原子保存回滚；SQLite 同时通过，外部用例无跳过。回环 gRPC/Gateway 通过；两个测试项目的容器、卷及网络经 Docker 标签过滤只读检查均为零。日志仅作临时证据，正式复现入口见 README。
- 2026-10-11 00:08 D1 SQLite：`CGO_ENABLED=0` 本机测试通过多连接 PRAGMA、显式 Goose 迁移及重复执行、唯一/外键错误、事务回滚、运行中 deadline、busy timeout 与连接关闭后持久化读取；策略原型通过缺表失败且不建表、默认拒绝、角色权限、重载、删除与保存失败回滚。运行依赖树只包含 libtnb/modernc SQLite，不含第二套驱动。
- 2026-10-10 23:16 文档与仓库基线：Python 检查 7 份 Markdown 的本地链接、参考名称排除、CLI 参数和能力覆盖；Git check-ignore 验证 31 个应忽略/应跟踪路径；确认 `main`、无暂存文件和远端，全部通过。尚无代码，不涉及构建、数据库或业务运行验证。
