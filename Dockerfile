# 多阶段构建：Go 编译 → 精简运行镜像
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/mian ./cmd

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /out/mian /app/mian
RUN mkdir -p /app/data
ENV MIRROR_DB_PATH=/app/data/mirror-mian.db
ENTRYPOINT ["/app/mian"]
# 启动 Web 服务（cmdWeb 默认 :8080，这里显式 :8012 便于隧道/DNS 指向）
CMD ["web", "-addr", ":8012"]
