FROM --platform=$BUILDPLATFORM golang:1.23.2 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY=https://proxy.golang.org,direct

COPY . /src
WORKDIR /src

RUN apt update && \
    apt install -y protobuf-compiler

RUN GO111MODULE=on GOPROXY=$GOPROXY go install github.com/google/wire/cmd/wire@v0.6.0 && \
    GO111MODULE=on GOPROXY=$GOPROXY go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest && \
    GO111MODULE=on GOPROXY=$GOPROXY go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.35.1 && \
    GO111MODULE=on GOPROXY=$GOPROXY go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

RUN go run ./cmd/tool/generate_proto.go api && \
    go run ./cmd/tool/generate_proto.go conf && \
    go generate transmission-proxy/cmd

RUN mkdir -p /src/bin/
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH GOPROXY=$GOPROXY go build \
            -ldflags "-X main.Version=`git describe --tags --always`" \
            -o ./bin/app  \
            ./cmd

FROM linuxserver/transmission:4.0.6

RUN apk --no-cache add nftables && \
    rm -rf /var/cache/apk/*

COPY --from=builder /src/bin/app /usr/sbin/trproxy
RUN chmod 755 /usr/sbin/trproxy

EXPOSE 8000
