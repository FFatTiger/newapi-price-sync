FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/newapi-price-sync ./cmd/newapi-price-sync

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/newapi-price-sync /usr/local/bin/newapi-price-sync
COPY config.example.yaml /app/config.yaml
ENTRYPOINT ["/usr/local/bin/newapi-price-sync"]
CMD ["--config", "/app/config.yaml"]
