FROM golang:1.27.3-alpine3.24 AS builder

WORKDIR /app

COPY go.sum go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/bin/neocollects-api ./cmd/api
# The tool of scripts/backup-all.sh and scripts/restore-all.sh when they run on a server (no Go there).
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/bin/neocollects-bundle ./cmd/bundle

FROM alpine:3.24

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/bin/neocollects-api /app/bin/neocollects-api
COPY --from=builder /app/bin/neocollects-bundle /app/bin/neocollects-bundle
COPY --from=builder /app/migrations ./migrations

USER 65534:65534

EXPOSE 8090
ENTRYPOINT ["/app/bin/neocollects-api"]
