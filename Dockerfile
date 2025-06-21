# hadolint ignore=DL3007
FROM marcaureln/volta:latest AS web-builder
WORKDIR /app/lumine/web
COPY ./lumine/web/package.json package.json
COPY ./lumine/web/pnpm-lock.yaml package-lock.yaml
RUN volta install node@lts corepack && pnpm install --no-frozen-lockfile --prod
COPY . ../../
RUN ../../install.sh --bin "/bin" --no-ayaka --no-ayato --no-lumine-go

FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY --from=web-builder /app/lumine/embed/out /app/lumine/embed/out
# hadolint ignore=DL3018
RUN apk add --no-cache binutils upx
COPY . .
RUN ./install.sh --bin "/bin" --no-lumine-web --upx

# hadolint ignore=DL3007
FROM alpine:latest
# hadolint ignore=DL3018
RUN apk add --no-cache pacman
COPY --from=builder /bin/ayaka /usr/local/bin/ayaka
COPY --from=builder /bin/ayato /usr/local/bin/ayato
COPY --from=builder /bin/lumine /usr/local/bin/lumine
