FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o video-optimization ./cmd/video-optimization

FROM alpine:latest

RUN apk add --no-cache ffmpeg ca-certificates

WORKDIR /app

COPY --from=builder /app/ .

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
  CMD curl --fail http://localhost:8080/health || exit 1

CMD ["./video-optimization"]
