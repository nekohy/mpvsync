# mpvsync

使用P2P的Syncplay插件，同步暂停、播放以及视频进度
在使用前请确保双方加载同一个播放列表
适用于 libmpv 类播放器（mpv,hills等）

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
