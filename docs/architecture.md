# 架构

Palworld Companion 0.4.0-dev 是单体、自托管应用。Vue PWA 嵌入纯 Go 二进制；账号、Session 和任务属于 Companion 自身 SQLite，浏览器不会接触 Palworld REST API 凭据。

```mermaid
flowchart LR
    P["手机浏览器 / PWA"] -->|"同源 HTTPS /api/v1"| C["Palworld Companion"]
    C -->|"只读 Basic Auth /info /metrics /players"| R["Palworld REST API"]
    C --> M["短时状态缓存"]
    C --> S["SQLite WAL"]
    C --> E["嵌入式 Vue 静态资源"]
```

认证流程没有外部 Steam 节点。新注册按实时 `/players` 中区分大小写的完整角色名查找唯一在线角色，再严格解析 `userId=steam_<SteamID64>`；旧 `/api/v1` 客户端仍可提交 SteamID64。

## 后端模块

- `internal/config`：YAML、兼容字段、默认值和持续时间校验。
- `internal/palworld`：只读客户端与绕过状态缓存的身份绑定读取。
- `internal/roster`：严格快照协调、SQLite 名册、缓存复用与公共玩家 DTO。
- `internal/serverstatus`：服务器 info、metrics、summary 与 roster 在线人数聚合。
- `internal/auth`：Argon2id、本地登录、首任初始化、玩家申请、审批状态、Session 和管理员保护。
- `internal/storage`：纯 Go SQLite、WAL、外键、busy timeout 和版本化事务迁移。
- `internal/tasks`：个人/共享任务和对象级权限。
- `internal/httpapi`：API、受控错误、进程内限速、安全响应头和 SPA。
- `cmd/companion`：服务入口及只从 TTY 读取密码的恢复 CLI。

## schema 4 认证基础

迁移 4 安全重建 `users`，保留原有主键，并增加：

- 可空且大小写不敏感唯一的 `username`
- 可空 `password_hash`，兼容没有本地密码的旧用户
- `pending/active/disabled/rejected/deleted` 状态
- `approved_at/by`、`rejected_at/by`、`rejection_reason`
- 删除前状态，用于软删除后的正确恢复
- `system_settings.setup_completed`

迁移复制 schema 3 用户后恢复并检查外键，因此 Session、任务 `owner_id/created_by` 和可见性保持不变。旧库有管理员时 `setup_completed=true`，否则为 false。初始化完成后不再根据管理员数量重新计算。

## schema 5 与玩家名册

schema 5 新增 player_roster，以 palworld_user_id 为唯一稳定身份，非空 palworld_player_id 也受部分唯一索引保护。表只保存角色名、等级、上次已知在线状态、首次发现、最后在线和更新时间；不保存 ping、坐标、accountName、IP 或原始响应。迁移只从已绑定且身份与角色名非空的 users 回填，空 playerId 转为 NULL，并且不会伪造 player_roster_last_success_at。

internal/roster 在事务外请求并校验整份新鲜快照，在同一 SQLite 事务中更新名册、绑定用户和最后成功时间。普通请求用互斥锁复用约 3 秒的成功缓存；缓存命中不写库。上游或校验失败时只读 SQLite，公共状态统一为 unknown，保留 lastKnownStatus，不修改 is_online 或 last_online_at。summary 和玩家接口共享同一 roster Service，角色名注册强制绕过 TTL 且禁止 stale/SQLite 身份匹配。

本版本没有后台常驻轮询；首页、summary、玩家接口和角色名注册触发更新。因此最后在线是 Companion 最后一次在完整成功快照中发现玩家在线的时间，不是精确在线时长。

## 认证与 Session

初始化管理员、设置完成标志和首个 Session 在同一数据库事务中创建。用户名使用 SQLite `COLLATE NOCASE` 唯一索引；SteamID64 必须为非零 uint64 十进制字符串。玩家登录可使用本地唯一角色名或 SteamID64，管理员使用本地用户名。

密码采用 Argon2id PHC 编码，验证使用恒定时间比较并限制编码参数，防止异常哈希触发不受控资源消耗。Session 使用 256 位随机 Token；客户端 Cookie 为 Secure、HttpOnly、SameSite=Lax、Path=/，数据库只保存 SHA-256。

注册身份读取通过共享 roster Service 强制请求新鲜 `/players`，不经过成功 TTL 或失败合并缓存。角色名必须精确且唯一，身份必须来自严格的 `steam_<uint64>` userId；注册失败不会回退过期结果。登录只查询本地 SQLite，已有账号离线或 Palworld API 故障时仍可登录；重复角色名统一返回无效凭据并允许改用 SteamID64。

## 权限与隐私

公开玩家 DTO 只包含角色名、等级、在线/未知状态、最后在线时间，以及仅在当前新鲜在线快照中存在的 ping/position。SteamID64、Palworld userId/playerId、accountName、IP 和数据库 ID 仅出现在当前账号或管理员接口，不进入公共玩家响应。

任务查询在 SQL 层按 actor 和 visibility 过滤，并由 service 重复校验管理权限。玩家无法读取其他人的个人任务；共享任务只有创建者或管理员能写；无权对象统一返回 404。

管理员写接口重新验证当前 active Session 与 admin 角色。当前管理员不能禁用或删除自己；最后一个 active 管理员不能被禁用、删除或降级。禁用、删除和密码重置都会撤销目标 Session。

Service Worker 的 navigation fallback 拒绝整个 `/api/`，没有认证、初始化、注册、任务或管理员响应进入缓存。
