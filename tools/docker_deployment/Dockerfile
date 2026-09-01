FROM golang:1.26.4 AS builder

WORKDIR /app

RUN go install github.com/swaggo/swag/cmd/swag@latest && \
    go install golang.org/x/text/cmd/gotext@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN make swaggo-api-init translation changelog docs-templatedata env-example

RUN VERSION="$(date -u '+%Y')-$(git rev-list --count HEAD)-$(git rev-parse --short HEAD)" && \
    BUILD_TIME="$(date -u '+%Y-%m-%d %H:%M')" && \
    LAST_COMMIT_MSG="$(git log -1 --pretty=%s)" && \
    CGO_ENABLED=0 GOOS=linux go build \
      -ldflags "-X 'knov/internal/version.Version=$VERSION' -X 'knov/internal/version.BuildTime=$BUILD_TIME UTC' -X 'knov/internal/version.LastCommitMessage=$LAST_COMMIT_MSG'" \
      -o bin/knov ./

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata su-exec && \
    addgroup -g 1000 knov && \
    adduser -D -H -u 1000 -G knov knov

WORKDIR /app

COPY --from=builder --chown=knov:knov /app/bin/knov ./knov
COPY tools/docker_deployment/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV KNOV_LOG_LEVEL=info

EXPOSE 1324

ENTRYPOINT ["/entrypoint.sh"]
