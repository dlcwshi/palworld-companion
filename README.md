# Palworld Companion

**简体中文** | [English](README.en.md)

[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8)
![Vue](https://img.shields.io/badge/Vue-3-42b883)

Palworld Companion 是自托管、手机端优先的 Palworld 玩家辅助 PWA。Go 后端通过严格的只读白名单连接 Palworld REST API，在 SQLite 中保存 Companion 自身账号和任务，并把 Vue 前端嵌入单个可执行文件。

**当前仓库版本：0.3.0-dev。**不创建 v0.3.0 Tag 或正式 Release。

## 当前功能

- 服务器状态、核心指标与经过脱敏的在线玩家列表。
- 首次打开强制创建本地管理员账号。
- 管理员使用用户名和密码登录；玩家使用 SteamID64 和本地密码登录。
- 玩家注册时必须在线进入本 Palworld 服务器，后端实时匹配 `userId == "steam_" + SteamID64`，注册后等待管理员审批。
- 用户审批、拒绝、禁用、软删除、恢复、角色管理、Session 撤销和密码重置。
- 个人任务与共享任务，权限在 SQL 和 service 两层校验。
- 移动端 PWA、SQLite WAL、纯 Go Linux AMD64 单二进制和 systemd 部署。

SteamID64 只作为本服玩家身份标识。Steam OpenID 已停用，Companion 不访问 `steamcommunity.com`、Steam Web API 或外部认证代理。

## 认证流程

### 首任管理员

空数据库或从 schema 3 升级且没有管理员时，`GET /api/v1/setup/status` 返回 `setupRequired=true`。前端会把所有应用页面导向 `/setup`。首任管理员填写用户名、可选显示名称和密码；创建、`setup_completed=true` 与 Session 在数据库事务中完成，并发请求只有一个能成功。

初始化一旦完成就永久关闭，即使最后一个管理员异常也不会自动重开。恢复使用 CLI。

### 玩家申请

1. 玩家先进入本 Palworld 服务器并保持在线。
2. 在 `/register` 提交 SteamID64 和本地密码。
3. 后端直接调用新鲜 `/players`，不使用状态缓存或 stale fallback。
4. 精确匹配成功后创建 `role=player,status=pending` 的申请。
5. 管理员批准后玩家才能登录；active 玩家后续登录不依赖 Palworld API 或当前在线状态。

pending、disabled、rejected 和 deleted 账号均不能登录。重复 SteamID64、Palworld userId 或稳定的 playerId 不能绕过现有申请状态。

## 密码与 Session

- 密码使用 Argon2id、每个密码独立随机盐和带参数的 PHC 编码；长度为 8–128 字节。
- 不保存明文密码，不使用单独 SHA-256 作为密码哈希。
- Session 原始 Token 只存浏览器 Secure、HttpOnly、SameSite=Lax、Path=/ Cookie；SQLite 只保存 Token 的 SHA-256。
- 用户修改密码后保留当前 Session并撤销其他 Session；管理员重置密码会撤销目标用户所有 Session。
- 登录、注册、初始化、修改密码和重置密码接口有有限容量的进程内限速。

详见 [docs/security.md](docs/security.md) 与 [docs/architecture.md](docs/architecture.md)。

## 任务权限

- 玩家只能看到自己的个人任务，不能通过 ID 访问其他玩家的个人任务。
- 共享任务对全部已登录用户可见，仅创建者或管理员可修改。
- 管理员可管理全部任务；无权限访问统一返回 404。
- schema 4 不修改 `tasks.owner_id`、`created_by` 或 `visibility`，旧任务不会丢失。

## 快速开始

要求 Go 1.24+、Node.js/npm；不需要 Docker、CGO、外部数据库或 Steam 服务。

```powershell
cd frontend
npm.cmd ci
npm.cmd run build
cd ..
go test ./...
go run ./cmd/companion --config deploy/config.example.yaml
```

打开 <http://127.0.0.1:8091>。示例配置使用 Mock 模式和 `./data/companion.db`。

配置中的 `auth.enabled`、`public_base_url`、`admin_steam_ids` 仅为旧版本兼容字段，0.3.0-dev 会读取但不会用于 Steam 认证。仍使用 `auth.session_ttl` 控制 Session 有效期。

## API

公共与初始化：

- `GET /api/v1/health`
- `GET /api/v1/system/version`
- `GET /api/v1/system/capabilities`
- `GET /api/v1/server/summary`
- `GET /api/v1/server/players`
- `GET /api/v1/setup/status`
- `POST /api/v1/setup/admin`

认证：

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/change-password`
- `GET /api/v1/auth/me`
- `POST /api/v1/auth/logout`
- 旧 `GET /api/v1/auth/steam` 和 callback 固定返回 `410 steam_auth_disabled`

管理员：

- `GET /api/v1/admin/users?status=pending|active|disabled|rejected|deleted`
- `POST /api/v1/admin/users/{id}/approve|reject|reset-password|role`
- `POST /api/v1/admin/users/{id}/disable|enable|restore|revoke-sessions`
- `DELETE /api/v1/admin/users/{id}`（软删除）

任务：

- `GET|POST /api/v1/tasks`
- `GET|PATCH|DELETE /api/v1/tasks/{id}`

## 恢复 CLI

密码必须从交互式 TTY 无回显读取，不能通过命令行参数传递：

```bash
palworld-companion setup status --config /etc/palworld-companion/config.yaml
palworld-companion users create-admin --config /etc/palworld-companion/config.yaml --username <username>
palworld-companion users approve --config /etc/palworld-companion/config.yaml --steam-id <SteamID64>
palworld-companion users reject --config /etc/palworld-companion/config.yaml --steam-id <SteamID64>
palworld-companion users reset-password --config /etc/palworld-companion/config.yaml --steam-id <SteamID64>
palworld-companion users reset-password --config /etc/palworld-companion/config.yaml --username <username>
```

无 TTY 时需要密码的命令会安全失败。

## 数据库升级与回滚

schema 4 增加本地用户名、Argon2id 密码哈希、审批/拒绝审计字段和持久化 `system_settings.setup_completed`。schema 3 用户、Session、任务和用户 ID 原地保留；旧 Steam 用户没有密码时不能无密码登录，管理员可为其重置密码。程序拒绝打开比自身更新的 schema。

升级前停止 Companion 并备份二进制、配置、`companion.db` 及 WAL/SHM。迁移失败会回滚事务并拒绝启动。回滚程序时必须同时恢复升级前数据库文件，不能让旧二进制打开 schema 4。

完整步骤见 [docs/deployment.md](docs/deployment.md)。

## 安全边界

后端只调用 Palworld `/info`、`/metrics`、`/players`；前端不会收到 REST 凭据、玩家 IP 或原始响应。Companion 不读写存档、不依赖 PST 数据库、不修改 Palworld 配置。真实配置、密码、数据库、Cookie、Token 和玩家私密标识不得提交 Git。

## 许可证

原创源代码采用 [MIT License](LICENSE)。第三方数据和素材遵守各自许可证，见 [NOTICE](NOTICE)。本项目与 Pocketpair, Inc. 无隶属、授权或背书关系。
