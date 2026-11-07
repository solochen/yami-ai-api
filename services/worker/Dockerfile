FROM golang:1.25-alpine AS builder
WORKDIR /app/services/worker
ENV GOPROXY=https://goproxy.cn,direct
ENV GOMAXPROCS=2
COPY services/modelcontract /app/services/modelcontract
COPY services/worker/go.mod services/worker/go.sum ./
RUN go mod download
COPY services/worker .
RUN CGO_ENABLED=0 go build -p=2 -o /worker ./cmd/worker

FROM alpine:3.20
# 提取商品图片会用本机浏览器打开用户提交的京东商品页。
RUN apk add --no-cache ca-certificates ffmpeg font-noto-cjk chromium
ENV CHROME_PATH=/usr/bin/chromium-browser
COPY --from=builder /worker /worker
CMD ["/worker"]
