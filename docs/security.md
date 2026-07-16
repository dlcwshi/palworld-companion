# 安全说明

## 持久化名册与故障安全

公共玩家 DTO 只包含角色名、等级、当前状态、上次已知状态、最后在线时间，以及仅在当前状态可确认且玩家在线时出现的实时 ping/坐标。SteamID64、palworld_user_id、palworld_player_id、accountName、IP、数据库 ID 和原始 /players 响应均不会序列化到公共接口。

/players 必须存在数组字段且整份快照中的每个玩家都有严格的 steam_<uint64> userId、非空角色名、唯一 userId 和唯一非空 playerId。任何单项异常都会拒绝整份快照；请求失败、非法 JSON、字段缺失/null、重复身份或数据库事务失败均不会修改在线状态、最后在线时间或最后成功同步时间。SQLite 回退只展示历史名册，当前状态统一标为 unknown，旧 ping 和坐标不会返回。

角色名不是稳定身份，不用于合并玩家。改名通过稳定 palworld_user_id 更新同一名册行；角色名注册继续强制请求新鲜快照，不能从持久化名册或 stale 缓存自动绑定。

## 密码

- 接受 8–128 字节密码，不强制字符组合。
- 使用 Argon2id、19 MiB 内存、2 次迭代、单并行度、16 字节随机盐和 32 字节输出。
- PHC 字符串保存算法版本和参数，以支持未来升级。
- 验证时限制内存、迭代、并行度、盐和输出长度，并使用恒定时间比较。
- 密码、确认密码、密码哈希和完整认证请求体不会写入日志。

## Session

Session Token 由 `crypto/rand` 生成，原始值只进入 Secure、HttpOnly、SameSite=Lax、Path=/ Cookie。数据库保存 SHA-256 哈希。注销、禁用、删除、管理员重置密码和显式撤销均设置 `revoked_at`。

用户修改密码会验证当前密码、更新 Argon2id 哈希并撤销除当前 Token 之外的 Session。管理员重置密码不读取旧密码或旧哈希，并撤销全部目标 Session。

## 初始化

`system_settings.setup_completed` 是唯一初始化开关。创建首任管理员时在事务内重新检查该值；成功后永久设置为 true。删除或损坏管理员不会重开 Web 初始化，CLI `users create-admin` 用于受控恢复。

## 注册与身份

新玩家注册接受角色名和密码；为了 `/api/v1` 向后兼容，仍接受 SteamID64 和密码，但两种标识必须二选一。角色名会去除首尾空白、限制为 1–80 个 Unicode 字符并拒绝控制字符。注册实时调用 `/players`，只接受区分大小写的唯一完整角色名，禁止状态缓存、stale fallback 或持久化名册匹配。

角色名路径只从匹配玩家的严格 `userId=steam_<非零 uint64>` 解析 SteamID64，不根据 playerId 猜测身份。数据库对 SteamID64、Palworld userId 和非空 playerId 建立唯一索引；角色名不是身份键，重名登录统一失败并可改用 SteamID64。公开注册响应不返回 SteamID64、userId、playerId、accountName 或原始玩家对象。

## 管理员与任务

管理员保护在事务中统计 active 管理员，阻止自禁用、自删除以及最后一个 active 管理员被禁用、删除或降级。普通玩家不能调用审批、密码重置或角色接口。

个人任务只对所有者和管理员可见；共享任务对所有登录用户可见，但只允许创建者和管理员修改。无权对象按不存在处理。

## 限速与错误

登录、注册、初始化、修改密码和重置密码使用有限容量、每 IP 的进程内窗口限速。错误响应只包含稳定 code 和安全 message；SQL、路径、内部堆栈、上游凭据和完整玩家对象不会返回。

## 已停用的 Steam OpenID

旧 Steam 路由固定返回 HTTP 410 和 `steam_auth_disabled`。运行时没有 OpenID verifier 或外部 HTTP 客户端装配，不会向 `steamcommunity.com` 或 Steam Web API 发请求。旧 `auth_flows` 表和兼容配置字段暂时保留，不参与认证。
