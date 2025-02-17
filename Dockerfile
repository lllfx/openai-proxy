FROM golang:1.21.1 AS builder

COPY ./ /app
WORKDIR /app

RUN go build -o main main.go
RUN chmod +x /app/main

FROM debian:bookworm-slim
COPY --from=builder /app/main /app
RUN apt-get update -y && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/* && update-ca-certificates

CMD ["/app/main"]