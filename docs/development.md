# 开发与环境恢复

本文提供仓库源码导航、新电脑恢复步骤和开发验收入口。项目当前进度只维护在 [ROADMAP](../ROADMAP.md)；完整建设顺序见 [施工清单](implementation.md)，长期工程规则见 [AGENTS](../AGENTS.md)。克隆仓库即可获得继续开发所需的规范，不依赖原电脑的聊天记录或本地缓存。

## 新电脑准备

| 工具 | 用途 | 要求 |
|---|---|---|
| Git | 克隆、版本管理 | 保留完整提交历史 |
| Go | 构建生成器与生成项目 | 固定 1.26.4，与 go.mod 和锁文件一致 |
| Make | 仓库检查入口 | 使用现有 Makefile |
| Python | 契约检查、协议工具与服务验收脚本 | 3.9 或以上 |
| curl | 下载固定协议工具 | 协议工具安装时需要 |
| OpenSSL | 生成隔离测试证书 | 数据库、Redis 与 TLS 验收需要 |
| Docker + Compose | 真实数据库、Redis 与容器验收 | 显式运行对应目标时需要 |

协议工具下载脚本当前提供 macOS/Linux 的 amd64/arm64 固定资产。Windows 可通过 WSL2 使用 Linux 开发路径；原生 Windows 的全部脚本工作流尚未验证。兼容性测试的 Windows 交叉编译不代表整个开发工具链在 Windows 原生可用。

首次恢复需要访问 Go 模块代理和固定工具下载源。使用本机正常的 Go 环境，不复制原电脑的 GOROOT 或 Go 安装目录。若有全局 GOROOT 覆盖，先核对它与实际执行的 Go 二进制一致。

```sh
# 在仓库上传后，使用实际 GitHub 仓库地址克隆
git clone <repository-url> goseed
cd goseed
git status --short --branch
git log -1 --oneline

go version
go env GOROOT GOPROXY GOTOOLCHAIN
python3 --version
make --version

# 基础文档检查与编译
make contract-check
make build
./bin/goseed --help
./bin/goseed --version

# 恢复项目内开发工具，不进行全局安装
make tools
make protocol-tools

# 快速核对核心实现，不启动外部服务
make test GO_PACKAGES='./internal/cli ./internal/generator ./internal/openapi ./internal/templates'
make vet
make lint
```

Makefile 固定 GOTOOLCHAIN=go1.26.4、GOWORK=off；Go 模块与构建缓存使用仓库的 `.tmp/`。首次下载失败先检查网络、代理和工具链配置，不通过升级版本或删除 go.sum 绕过失败。主模块仅直接依赖 Cobra 与 YAML 库，生成业务项目的完整运行依赖从已提交的模板 manifest 交付。

`.tools/`、`.tmp/`、`bin/` 和 Docker 测试资源不入 Git，属于可重建本地产物。新电脑重新安装工具、下载缓存即可，不能让代码依赖旧电脑的缓存存在。真实配置、私钥、数据库和未提交业务项目如需迁移，应单独安全保存，不能放进公开仓库。

## 文档阅读顺序

1. [项目规范](../AGENTS.md)和 [ROADMAP](../ROADMAP.md)：工程边界、真实状态与下一任务。
2. [产品需求](requirements.md)：用户场景、首版范围、默认行为与验收标准。
3. [技术架构](architecture.md)：目录、分层、组装、配置、日志与生命周期。
4. [命令契约](cli.md)：项目生成、业务模块和资源字段规范。
5. [施工清单](implementation.md)：从零建设的依赖顺序、修改范围和失败路径。
6. [资源接入方案](resource-generation.md)：当前业务生成阶段的接口、文件归属和兼容决策。
7. [依赖核验](dependencies.md)：固定版本、历史核验证据与已知上游限制。

依赖证据的日期是核验时间，不能解释为持续更新的维护保证。升级依赖时重新核验并同步锁文件、模板 manifests、工具脚本及对应测试。

## 源码导航与实现细节

| 关注点 | 入口 | 实现边界 |
|---|---|---|
| CLI | `cmd/goseed/main.go`、`internal/cli/root.go` | 解析参数、输出错误与计划，不承载业务运行逻辑 |
| 初始项目生成 | `internal/generator/generator.go` | 校验目录、规划完整文件、冲突保护和写入 |
| 模板组合 | `internal/templates/render.go` | embed 基础/变体源码，展开 module/name，格式化 Go，裁剪依赖与配置 |
| 项目状态 | `internal/generator/project.go` | 严格读取 goseed.yaml，核对登记与 OpenAPI 摘要，拒绝非法来源 |
| 普通模块增量 | `module_plan.go`、`module_assembly.go` | 一次性工厂与可再生成登记分开；重验输入，失败恢复本次变更 |
| OpenAPI | `internal/openapi/merge.go`、`openapi_plan.go` | Swagger 2.0 多源确定性合并、冲突拒绝、摘要与安全刷新 |
| 资源输入 | `resource_spec.go` | 版本 1 YAML，string/bool/int64；严格字段、重复键与保留名校验 |
| 资源计划 | `resource_plan.go`、`resource_tools.go` | 能力/工具/路径/输入快照检查；目前没有 Write 或公共 CLI |
| 资源 SQL | `resource_schema.go`、`resource_migration.go` | 当前方言建表草稿、下一版本计算；不连接或修改数据库 |
| 资源 CRUD | `resource_core.go`、`resource/core/` | Model、Biz、Store 与测试；按 owner 限定 SQL，保留 PATCH 零值 |
| HTTP 资源 | `resource/http*.tmpl` | 强类型 DTO、JSON/ID/query 校验、共享中间件；当前 HTTPFiles 返回源文件 |
| 真实生成验收 | `tests/integration/` | 独立项目 readonly 编译、race、vet、lint、进程与真实服务验证 |
| 依赖兼容原型 | `tests/compatibility/` | 独立 Go 模块；原型不等于正式业务模板实现 |

表中未写完整前缀的生成器文件位于 `internal/generator/`；资源模板与渲染器位于 `internal/templates/`。修改模板应核对基础层和覆盖变体，不能只修改一个变体便宣称所有组合一致。

生成服务的公共组件位于模板的 `internal/pkg/`，包括 config、logx、errno、contextx、database、middleware 与协议服务。数据库对象只在 Store/Bootstrap 使用，Biz 接收它实际需要的仓储接口。do 只用于启动组装，Viper 输出强类型配置，Zap 统一结构化日志。设计细节以架构和对应模板契约为准。

模块工厂 `bootstrap/module_<name>.go` 只创建一次，由开发者维护；`modules_gen.go`、goseed.yaml 与 Swagger 汇总是生成器维护的登记投影。模块纯权限描述不构造业务，也不连接数据库/Redis。组装入口版本 2 不等于完整资源兼容性版本，R09 还需验证启动 Context、schema 与所选协议入口。

迁移版本从嵌入 SQL 文件推导。启动只读核对完整应用集合，显式迁移仅接受已应用前缀；不会在启动时 AutoMigrate。每个资源工厂还需核对实际列、主键、长度约束与 owner 排序。MySQL DDL 失败可能保留部分 schema，不能沿用 SQLite/PostgreSQL 的回滚假设。

## 验证层次

| 入口 | 验证对象 | 环境与边界 |
|---|---|---|
| `make contract-check`、`git diff --check` | 文档链接、固定版本、Python 语法、文件格式 | 文档修改的必需检查 |
| `make test GO_PACKAGES='...'` | 指定核心包 | 不等于全组合验收 |
| `make check` | 全仓测试、vet、lint 与契约 | 需项目工具和本机回环监听；完整测试最长 40 分钟 |
| `make race` | 全仓竞争检查 | 普通外部服务用例缺凭据可能跳过 |
| `make compatibility`、`make compatibility-cross` | 依赖原型及跨平台构建 | 不等于业务模板生产部署验证 |
| `make protocol-check` | 已提交协议产物复现 | 需固定协议工具 |
| `make template-db` | 真 MySQL/PostgreSQL、TLS、策略、迁移及资源核心 | Docker/Compose/OpenSSL；显式目标不允许跳过外部用例 |
| `make template-redis` | 真 Redis/TLS、故障恢复与授权顺序 | 隔离服务、随机回环端口 |
| `make template-container` | 部署镜像、非 root、只读文件系统、TLS/生命周期 | 显式启动 Docker，普通检查不启动它 |

资源 SQLite 验收的定向命令：

```sh
make test GO_PACKAGES='./tests/integration -run TestResourceCoreProjects -count=1 -v'
```

该套件包含 HTTP 回环监听，受限制的执行环境须允许本机监听。Casbin/Redis 资源 HTTP 夹具使用接口替身；真实策略、真实缓存与生成命令全链路验收仍需按施工清单补齐，不能从该测试推断已经通过。

验证结束后清理本次创建的任务目录、日志与可重建产物，停止本次专用测试服务，并核对结果。中断测试可能跳过 cleanup；只处理能确认属于本次任务的对象，不清空整个 `.tmp/` 或共享服务。

## 从当前代码继续开发

以 ROADMAP 的状态为准，不重做已交付功能。先关闭 R06 文档与权限验收缺口，再执行 R07、R08、R09、R10，最后 D7。R07 的 Proto/dual 组装决策应沿用资源接入方案，新增 RPC 必须登记精确 FullMethod；普通构建继续使用已生成 Proto。

资源内部接口目前分开提供 CoreFiles、HTTPFiles、SchemaStore 和 MigrationDraft。SchemaStore 是早期只读原型，不能和 CoreFiles 的完整 Store 写入同一路径。HTTPFiles 当前为 dual 渲染的工厂仅含 HTTP 成员，完整 dual 工厂需由 R07 加入 RPC 并共享同一 Biz。R09 统一完整渲染、协议编译、登记/汇总更新和安全写入后，才能开放 gen resource。

每次任务先读规范与施工单，确认工作区和验证目标；实现后更新受影响文档、ROADMAP，必要验证通过后按功能本地提交。公开推送、发版与部署另有明确授权和目标。

## 公开仓库准备

本仓库采用 [MIT](../LICENSE)，第三方依赖与生成代码保留原有版权及许可说明，依赖范围见 [许可证记录](dependencies.md#许可证范围)。模板的整体许可证不替换第三方依赖许可证。

公开源码交付物为 Git 跟踪的源码、模板、测试、脚本、文档、锁文件和 LICENSE。`.gitignore` 已排除真实配置、数据库、证书、本地工具/缓存、构建产物和 AI 本地配置；忽略规则不会移除已经跟踪或写进历史的文件。

推送前核对：

- 工作区和提交只包含本项目交付物；配置示例没有真实凭据。
- 当前文件与准备公开的 Git 历史均检查令牌、私钥和个人机器绝对路径；测试字符串与实际凭据分辨处理。
- 第三方生成文件的版权头保留；需要核验来源的内容不能靠删去名字解决许可问题。
- Git 作者身份属于公开历史，检查是否符合个人公开偏好；不自动改写作者或历史。
- 远端地址、可见性和首次推送分支确定；本地检查不代表已经发布。
- 确認 GitHub 上的 main 包含最新文档提交，再在新电脑克隆；未推送的本地提交不会随克隆恢复。

如尚未创建远端，先创建空仓库，避免远端生成 README/许可证与现有历史产生不必要冲突。仓库上传后再核对实际远端状态；不以未来地址或本地 module path 证明仓库已经存在。
