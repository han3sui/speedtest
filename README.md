# 测速工具
请自行下载系统对应版本`xray`客户端放入`client`目录中，命名规则为`xray`
> 项目地址：https://github.com/XTLS/Xray-core/releases/

## 使用说明

```shell
-h 使用说明
-u vmess订阅链接
-s 筛选节点，多个节点用 | 分割，例如: 香港|美国，节点增加条件用 & 分割，例如：香港&planD|美国
-d 测速文件下载时长，单位/秒，默认为15秒
```

## 打包编译linux

```
set GOARCH=amd64
set GOOS=linux
go build -o dist/linux/speedtest main.go
```

## 打包编译windows

```
set GOARCH=amd64
set GOOS=windows
go build -o dist/windows/speedtest.exe main.go
```