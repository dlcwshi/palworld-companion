# Palworld Companion 开发规则

## 目标与边界

Palworld Companion 是自托管、移动端优先的 Palworld 玩家辅助 PWA。优先交付最小可用功能并保持 `/api/v1` 向后兼容。

## 技术栈

- Go 后端，Vue 3 + TypeScript 前端。
- SQLite 仅作为后续业务持久化预留；不引入 MySQL、PostgreSQL 或 Redis。
- 前端产物通过 `go:embed` 进入单个 Go 可执行文件。

## 强制规则

1. 编辑前确认当前目录并读取最近的 AGENTS.md。
2. 只在本项目目录内修改文件，保持移动端优先。
3. 不使用 Docker，不拆微服务。
4. 不向前端暴露 Palworld REST API、凭据、原始响应或透明代理。
5. Palworld 客户端只允许调用 `/info`、`/metrics`、`/players` 三个明确白名单只读接口。
6. 不修改或解析 Palworld 存档，不修改 Palworld/PST 配置、数据库或服务，不执行 `clean-seeds`。
7. 普通安装依赖、编译、测试和本地模拟运行可以直接执行。
8. API 变更应保持向后兼容；破坏性变更必须先说明。
9. 游戏静态数据必须记录来源、版本和许可证；不提交来源不明或官方提取素材。
10. 不提交真实凭据、认证 Header、玩家 IP、平台账号或其他玩家隐私。