# 部署说明

本文件描述 Linux AMD64 部署方式，并记录不含凭据或玩家隐私的部署基线。

## 产物

目标基线为 Ubuntu 24.04 x86_64，不使用 Docker。构建机需要 Go 与 Node.js；最终服务器只需要单个二进制和 YAML 配置。

```bash
make build-linux
```

推荐路径：

- 程序：`/usr/local/bin/palworld-companion`
- 配置：`/etc/palworld-companion/config.yaml`
- 数据：`/var/lib/palworld-companion`
- unit：`/etc/systemd/system/palworld-companion.service`

## 服务与安全

使用独立低权限账户 `palworld-companion`。该账户不应拥有 Palworld、PST 或存档目录写权限。示例 unit 只给 `/var/lib/palworld-companion` 写权限。

从 `deploy/config.example.yaml` 复制配置，使用文件权限保护真实用户名和密码。默认监听 `127.0.0.1:8091`。

v0.1.0 没有 Companion 自身账户系统。公网使用前必须由 HTTPS 反向代理增加访问认证和速率限制，并保持 Companion 与 Palworld REST API 端口不直接暴露。部署、安装 unit 与重启服务应作为独立的明确授权任务执行。

## 2026-07-15 部署记录

- 目标：`192.168.3.113`，Ubuntu 24.04.4 LTS x86_64。
- 版本：`v0.1.0`，源码提交 `9dd9777`。
- Linux AMD64 产物 SHA-256：`8d9c66e89e9a99531255de0fa20aa1f29073df13c4d17d9e393d9d7f28cad306`。
- 服务：`palworld-companion.service` 已启用并运行，仅监听 `127.0.0.1:8091`。
- 上游：通过回环地址 `127.0.0.1:8212` 读取真实 `/v1/api/info`、`/metrics`、`/players`，`mock_mode=false`。
- 实际字段兼容：`serverfps` 为整数；`serverfpsaverage`、`serverframetime`、玩家延迟与二维坐标为浮点数；人数、运行时间、世界天数、基地数量与玩家等级为整数。当前公开模型所需字段均可正常解析。
- 隐私检查：公开玩家对象仅含 `name`、`level`、`ping`、`position`；原始 `iP`、`playerId`、`userId`、`accountName` 未向 Companion API 返回。
- 故障检查：使用临时回环测试实例和未监听的上游端口验证；上游失败时 health 与页面仍可用，数据接口返回明确错误，测试实例与配置已清理。
- 影响边界：部署前后 Palworld 与 PST 的主进程、启动时间和重启计数保持不变；未修改或重启其服务、配置或存档。

服务器现有 Palworld REST API 监听为 `0.0.0.0:8212`。这是 Companion 部署前已存在的状态，本次任务未修改 Palworld 配置、启动参数或防火墙；Companion 自身仍固定使用回环地址连接。
