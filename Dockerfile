# 使用 Go 官方镜像作为构建环境
FROM golang:alpine AS builder

# 设置 Go 代理（国内镜像）
ENV GOPROXY=https://goproxy.cn,direct

# 设置工作目录
WORKDIR /app

# 复制所有文件
COPY . .

# 编译
RUN go build -o aim .

# 使用更小的镜像运行
FROM alpine:latest

# 安装 ca-certificates（HTTPS请求需要）
RUN apk --no-cache add ca-certificates

# 设置工作目录
WORKDIR /app

# 从构建阶段复制可执行文件
COPY --from=builder /app/aim .
COPY --from=builder /app/uploads ./uploads

# 暴露端口
EXPOSE 8080

# 运行
CMD ["./aim"]
