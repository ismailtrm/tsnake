FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/tsnake .

FROM alpine:3.22

RUN adduser -D -h /app app && \
    apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /out/tsnake /app/tsnake
RUN mkdir -p /app/data && chown -R app:app /app

USER app

EXPOSE 2222
VOLUME ["/app/data"]

CMD ["./tsnake", "-mode=ssh", "-addr=:2222", "-host-key-path=/app/data/host_key"]
