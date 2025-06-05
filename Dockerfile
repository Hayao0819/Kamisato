FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN cd ayaka && go build -o /bin/ayaka ./main.go
RUN cd ayato && go build -o /bin/ayato ./main.go
RUN cd lumine && go build -o /bin/lumine ./main.go

FROM alpine:latest
COPY --from=builder /bin/ayaka /usr/local/bin/ayaka
COPY --from=builder /bin/ayato /usr/local/bin/ayato
COPY --from=builder /bin/lumine /usr/localbin/lumine
