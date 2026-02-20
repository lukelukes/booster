FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /booster ./cmd/cli

FROM archlinux:latest
RUN pacman -Sy --noconfirm tmux sudo
COPY --from=builder /booster /usr/local/bin/booster
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
WORKDIR /workspace
COPY testdata/ /workspace/testdata/
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
