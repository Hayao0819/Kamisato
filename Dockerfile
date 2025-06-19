FROM marcaureln/volta:latest as web-builder
WORKDIR /app
RUN mkdir -p lumine/web
COPY ./lumine/web/package.json lumine/web/package.json
COPY ./lumine/web/pnpm-lock.yaml lumine/web/package-lock.yaml
RUN volta install node@lts
RUN volta install corepack

RUN cd ./lumine/web && pnpm install --no-frozen-lockfile --prod
COPY . .
RUN ./install.sh --bin "/bin" --no-ayaka --no-ayato --no-lumine-go

FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY --from=web-builder /app/lumine/embed/out /app/lumine/embed/out
COPY . .
RUN ./install.sh --bin "/bin" --no-lumine-web

FROM alpine:latest
COPY --from=builder /bin/ayaka /usr/local/bin/ayaka
COPY --from=builder /bin/ayato /usr/local/bin/ayato
COPY --from=builder /bin/lumine /usr/local/bin/lumine
