# hadolint ignore=DL3007
FROM marcaureln/volta:latest AS web-builder
WORKDIR /app/lumine/web
COPY ./lumine/web/package.json package.json
COPY ./lumine/web/pnpm-lock.yaml package-lock.yaml
RUN volta install node@lts corepack && pnpm install --no-frozen-lockfile --prod
COPY . ../../
RUN ../../install.sh --bin "/bin" --no-ayaka --no-ayato --no-miko --no-lumine-go

FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY --from=web-builder /app/lumine/embed/out /app/lumine/embed/out
# hadolint ignore=DL3018
RUN apk add --no-cache binutils upx
COPY . .
# Builds standalone ayaka/ayato/lumine plus the combined `kamisato` binary
# (exposing `kamisato {ayato,miko,ayaka,lumine}`) for compose/terraform.
RUN ./install.sh --bin "/bin" --no-lumine-web --kamisato --upx

# hadolint ignore=DL3007
FROM alpine:latest
# pacman: ayato repo-add. git: miko build source materialization.
# hadolint ignore=DL3018
RUN apk add --no-cache pacman git
COPY --from=builder /bin/kamisato /usr/local/bin/kamisato
COPY --from=builder /bin/ayaka /usr/local/bin/ayaka
COPY --from=builder /bin/ayato /usr/local/bin/ayato
COPY --from=builder /bin/miko /usr/local/bin/miko
COPY --from=builder /bin/lumine /usr/local/bin/lumine
