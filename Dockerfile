FROM --platform=$BUILDPLATFORM golang:1.18 as builder
ARG TARGETOS TARGETARCH
WORKDIR /workspace
ENV GOPROXY=https://goproxy.cn,direct
COPY . ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -o echo-server-$TARGETARCH .

FROM alpine:3.17
RUN set -ex \
    && apk update \
    && apk upgrade \
    && apk add --no-cache \
    bash \
    curl
    
ARG TARGETARCH
WORKDIR /
ADD /ctl/qappctl-linux-$TARGETARCH /usr/bin/qappctl
COPY --from=builder /workspace/echo-server-$TARGETARCH /echo-server

ENTRYPOINT ["/echo-server"]