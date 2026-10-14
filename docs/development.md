# 开发指南

本文面向 goseed 贡献者。普通使用请从仓库根目录 [README](../README.md) 开始；命令和生成行为以 [CLI 参考](cli.md)、[模板指南](templates.md) 与生成项目自身的 README 为准。

## 本地开发

需要 Go 1.26.4 或更新的兼容版本、Git、Make 和 Python 3.9+。数据库、Redis、协议生成或容器验收时，再按相应目标准备 Docker、Compose、OpenSSL 和协议工具。依赖工具安装在仓库 `.tools/`，构建缓存和临时项目放在 `.tmp/`。

```sh
# 构建及文档/依赖契约检查
make build
make contract-check

# 安装仓库级测试和协议工具
make tools
make protocol-tools

# 全仓测试、静态检查和生成项目验收
make check
```

`make check` 会运行独立生成项目测试，可能需要较长时间。按改动范围可使用定向测试：

```sh
make test GO_PACKAGES='./cmd/goseed ./internal/cli ./internal/generator'
make vet GO_PACKAGES='./cmd/goseed ./internal/cli ./internal/generator'
make protocol-check
```

真实服务验收使用隔离数据库、Redis 和容器，只在显式执行对应目标时启动：

| 目标 | 验证内容 |
|---|---|
| `make template-db` | 真实 MySQL/PostgreSQL、迁移、TLS 和资源数据库行为 |
| `make template-redis` | Redis/TLS、缓存故障处理及授权顺序 |
| `make template-container` | 生成容器的非 root、只读文件系统和服务行为 |
| `make template-deployment` | systemd/Nginx 服务启动、TLS、重启和停止行为 |

验证只使用本次专用测试资源；外部数据库与 Redis 用例不允许静默跳过。详细边界见 [模板指南](templates.md)、[部署指南](deployment.md) 和 ROADMAP 中的最近验证记录。

## 持续集成

[CI 工作流](../.github/workflows/ci.yml) 在推送 main、面向 main 的 PR 和手动触发时运行。使用 Ubuntu 24.04，Go 版本从 `go.mod` 读取；官方 Actions 以完整提交 SHA 固定，工作流仅申请仓库内容读取权限。

CI 检查格式与文档契约，安装锁定协议工具，执行生成器/CLI/模板单元测试、缓存位置定向集成回归、vet 和构建。缓存覆盖仓库实际使用的 `.tmp/go-cache` 与 `.tmp/mod-cache`；新 runner 首次构建需要下载模板运行依赖。普通 CI 不运行 Docker、外部数据库、race 或完整生成组合运行矩阵，不能代替这些能力的专项验收。

发布仍按施工计划的版本发布流程人工执行，CI 不自动创建标签或 Release。

## 源码导航

| 关注点 | 入口 |
|---|---|
| 命令解析 | `cmd/goseed/`、`internal/cli/` |
| 计划、冲突保护与写入 | `internal/generator/` |
| 基础模板和能力组合 | `internal/templates/` |
| 模块/资源生成与协议适配 | `internal/templates/resource/`、`internal/templates/resource/core/` |
| 生成结果验收 | `tests/integration/`、`tests/compatibility/` |

模板源码是唯一生成实现来源；修改模板后要验证实际渲染项目。业务项目不导入 goseed 内部代码。资源生成接口、权限目录、迁移和协议复现边界见 [资源生成设计](resource-generation.md)；分层和生命周期见[架构说明](architecture.md)。

## 维护资料

- [AGENTS.md](../AGENTS.md)：稳定工程边界、文档职责和验证要求。
- [ROADMAP.md](../ROADMAP.md)：当前进度、开发记录和最近验证。
- [施工计划](implementation.md)：阶段依赖、交付物和后续方向。
- [依赖记录](dependencies.md)：版本、来源、许可证和已知上游限制。

功能状态以 ROADMAP 为准。完成代码或文档改动后，更新受影响的使用说明与进度记录，并运行和改动相称的检查。
