# mpvsync

几台电脑一起看番用的 mpv 同步插件，谁暂停、播放、拖进度，其他人跟着动

- 点对点，不用服务器，互相填 IP 就行
- 不记状态，UDP 发一包算一包
- mpv 和读 mpv 配置目录的 libmpv 播放器都能用，比如 Hills

## 编译

需要 Go 1.22 以上

```sh
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" ./cmd/mpvsync
```

交叉编译加上 `GOOS=windows` 或 `GOOS=linux`，PowerShell 里这么写

```powershell
$env:CGO_ENABLED = 0; $env:GOOS = "windows"; go build -trimpath -ldflags "-s -w" ./cmd/mpvsync
```

## 安装

把编出来的 `mpvsync`（Windows 上是 `mpvsync.exe`）和 `mpv/` 里的两个文件夹一起放进 mpv 配置目录

```
<mpv 配置目录>/
├─ mpvsync(.exe)
├─ scripts/mpvsync.lua
└─ script-opts/mpvsync.conf
```

| 播放器 | 配置目录 |
|---|---|
| mpv（Windows） | `%APPDATA%\mpv\`，或 `mpv.exe` 旁边的 `portable_config\` |
| mpv（Linux） | `~/.config/mpv/` |
| Hills 内置播放器 | `hills_data\mpv\` |

Linux 上记得 `chmod +x mpvsync`

## 配置

改 `script-opts/mpvsync.conf`

```
enabled=yes
peers=192.168.1.20,192.168.1.21
```

- `peers` 填其他人的 IP，逗号隔开，对方端口不是 7373 就写 `ip:端口`
- 暂时不想同步，把 `enabled` 改成 `no`

每台机器都要在防火墙放行 UDP 7373 入站

## 同步规则

- 暂停、播放、拖进度会发给所有人，看同一集的立刻跟上
- 落后正在播放的人 2 秒以上就自动跳过去，不会把人往回拽
- 刚打开一集的 10 秒内，别人正暂停着，就停到他的位置
- 是不是同一集看时长，差 0.5 秒以内算同一个文件，所以大家的片源要一样

## 已知问题

- 两个人几乎同时操作，可能各停各的，谁再点一下就对齐了
- 丢包不重发，进度差靠心跳补回来，暂停状态要等下一次操作

## 工作方式

`mpvsync.lua` 给播放器开一个 IPC 管道，再拉起 `mpvsync`

`mpvsync` 通过 IPC 盯着暂停、进度和跳转，用 UDP 把状态发给 peers，收到的操作再应用到本地播放器，播放器一关它就退出

```
cmd/mpvsync/    同步逻辑
internal/mpv/   mpv IPC 客户端
mpv/            放进 mpv 配置目录的脚本和配置
```

一台机器上开两个测试时用 `-port` 错开端口，比如 `mpvsync -ipc <管道> -port 7374 -peers 127.0.0.1:7373`
