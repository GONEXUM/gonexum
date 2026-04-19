# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG TMDB_API_KEY=""
ARG TMDB_PROXY_URL=""
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-s -w -X main.AppVersion=${VERSION} -X main.tmdbAPIKey=${TMDB_API_KEY} -X main.nexumTMDBBase=${TMDB_PROXY_URL}" \
    -o /gonexum-web ./cmd/gonexum-web/

# ── Runtime stage ────────────────────────────────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache mediainfo ffmpeg-libs ffmpeg

COPY --from=builder /gonexum-web /usr/local/bin/gonexum-web

# Config & data volumes
VOLUME ["/data", "/config"]

EXPOSE 8566

ENTRYPOINT ["gonexum-web", "--host", "0.0.0.0", "--port", "8566", "--config", "/config/settings.json", "--browse-root", "/"]
