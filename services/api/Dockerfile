FROM golang:1.25-alpine AS builder
WORKDIR /app/services/api
ENV GOPROXY=https://goproxy.cn,direct
ENV GOMAXPROCS=2
COPY services/modelcontract /app/services/modelcontract
COPY services/api/go.mod services/api/go.sum ./
RUN go mod download
COPY services/api .
RUN CGO_ENABLED=0 go build -p=2 -o /api ./cmd/api
RUN CGO_ENABLED=0 go build -p=2 -o /migrate ./cmd/migrate
RUN CGO_ENABLED=0 go build -p=2 -o /pricedrift ./cmd/pricedrift

FROM alpine:3.20
RUN apk add --no-cache ca-certificates ffmpeg python3 py3-pip \
    && pip install --break-system-packages --no-cache-dir yt-dlp
COPY --from=builder /api /api
COPY --from=builder /migrate /migrate
COPY --from=builder /pricedrift /pricedrift
EXPOSE 8080
CMD ["/api"]
