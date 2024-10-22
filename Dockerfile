FROM --platform=$BUILDPLATFORM golang:1.23.2 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY=https://goproxy.io

COPY . /src
WORKDIR /src

RUN apt update && \
    apt install -y protobuf-compiler && \
    GO111MODULE=on \
            go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest \
            go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.5.4 \
            go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1 \
    go run ./cmd/tool generate_proto.go api \
    go run ./cmd/tool generate_proto.go conf

RUN mkdir -p /src/bin/
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH GOPROXY=$GOPROXY go build \
    -ldflags "-X main.Version=`git describe --tags --always`" \
    -o ./bin/app  \
    ./cmd

FROM linuxserver/transmission:4.0.6

RUN apk --no-cache add \
    nftables \
    && rm -rf /var/cache/apk/*

COPY --from=builder /src/bin/app /usr/sbin/trproxy
RUN chmod 755 /usr/sbin/trproxy

EXPOSE 8000
