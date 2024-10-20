# transmission-proxy

将 Transmission 代理出一组 qBittorrent API 提供给 PeerBanHelper 与 AutoBangumi 使用

## 开发

### 初始化环境

#### 安装 Protocol Buffers

建议使用Protocol Buffers v3.20.2

##### 使用包管理器安装

```shell
apt install protobuf-compiler
```

##### 手动安装

[Protocol Buffers v3.20.2](https://github.com/protocolbuffers/protobuf/releases/tag/v3.20.2)

#### 初始化开发环境

```shell
go install github.com/go-kratos/kratos/cmd/kratos/v2@latest
go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.5.4
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest

go mod tidy
```

### 构建

#### 使用 proto 构建代码
```shell
go generate transmission-proxy/cmd/tool
```

#### 依赖注入
```shell
go generate transmission-proxy/cmd
```

#### 构建项目为可执行文件

```shell
go build -ldflags "-X main.Version=`git describe --tags --always`" -o ./bin/app ./cmd
```
