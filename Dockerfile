FROM --platform=$BUILDPLATFORM golang:1.23.2 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY=https://goproxy.io

COPY . /src
WORKDIR /src

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
