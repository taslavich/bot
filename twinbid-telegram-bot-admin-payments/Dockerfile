FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/twinbid-telegram-bot ./cmd/bot

FROM alpine:3.20
WORKDIR /app
RUN adduser -D -H appuser && mkdir -p /app/data && chown -R appuser:appuser /app
COPY --from=builder /out/twinbid-telegram-bot /app/twinbid-telegram-bot
USER appuser
EXPOSE 8090
CMD ["/app/twinbid-telegram-bot"]
