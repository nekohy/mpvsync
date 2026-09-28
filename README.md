# mpvsync

几台电脑一起看番用的 mpv 同步插件，谁暂停、播放、拖进度，其他人跟着动

点对点不用服务器，大家播同一个文件（大小一样）就会同步，mpv 和 Hills 这类 libmpv 播放器都能用

## 编译

```sh
go build ./cmd/mpvsync
```

## 安装

把 `mpvsync`（Windows 上是 `mpvsync.exe`）和 `mpv/` 里的 `scripts`、`script-opts` 一起放进 mpv 配置目录

- Windows：`%APPDATA%\mpv\`
- Linux：`~/.config/mpv/`
- Hills：`hills_data\mpv\`

## 配置

`script-opts/mpvsync.conf`

```
enabled=yes
peers=192.168.1.20,192.168.1.21
```

`peers` 填其他人的 IP，逗号隔开
