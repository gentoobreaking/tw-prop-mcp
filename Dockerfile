FROM golang:1.26-alpine3.24 AS builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/realestate-mcp ./cmd/realestate-mcp

FROM alpine:latest
RUN apk add --no-cache ca-certificates && addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=builder /out/realestate-mcp /usr/local/bin/realestate-mcp
USER appuser
EXPOSE 8080
ENTRYPOINT ["realestate-mcp"]
