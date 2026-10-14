FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM alpine:latest
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk --no-cache add ca-certificates tzdata
# 时区：默认 alpine 无 tzdata，time.Local 会回退到 UTC，导致写入/读出时间为 UTC。
# 装上 tzdata 并设 TZ=Asia/Shanghai，使 Go 的 time.Now()/time.Local 走北京时间。
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/config ./config
EXPOSE 8080
CMD ["./server"]
