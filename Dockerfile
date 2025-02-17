FROM golang:1.21.1 AS builder

COPY ./ /app
WORKDIR /app

RUN go env -w GO111MODULE=on
RUN go env -w GOPROXY=https://goproxy.cn,direct

RUN go build -o main main.go

FROM debian:bookworm-slim
#RUN set -x && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
#    ca-certificates && \
#    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/main /app/main
RUN chmod +x /app/main
CMD ["/app/main"]