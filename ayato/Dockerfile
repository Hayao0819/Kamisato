FROM golang:1.24-alpine as builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o ayato/ayato ./ayato/main.go

FROM archlinux/archlinux:base
WORKDIR /app
RUN pacman -Syu --noconfirm
COPY --from=builder /app/ayato/ayato /usr/local/bin/ayato
COPY --from=builder /app/ayato/ayato_config.json /app/ayato_config.json
COPY --from=builder /app/ayato/.env /app/.env

CMD [ "./ayato" ]
