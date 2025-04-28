FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN apk update && apk add --no-cache git && go mod download && apk del git
COPY . .
RUN go build -o balancer ./cmd/balancer

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/balancer .
RUN mkdir -p /app/conf /app/data
VOLUME ["/app/conf", "/app/data"]
EXPOSE 8080

ENTRYPOINT ["./balancer", "-config", "/app/conf/config.yaml"]

