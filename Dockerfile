# syntax=docker/dockerfile:1

# --- Stage 1: build the SPA -------------------------------------------------
FROM node:20-alpine AS web
WORKDIR /frontend
COPY cmd/aibreak-desktop/frontend/package.json cmd/aibreak-desktop/frontend/package-lock.json ./
RUN npm ci
COPY cmd/aibreak-desktop/frontend/ ./
RUN npm run build

# --- Stage 2: build aibreakd with the SPA embedded --------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /frontend/dist/ ./internal/webui/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/aibreakd ./cmd/aibreakd

# --- Stage 3: runtime -------------------------------------------------------
FROM alpine:3.20
RUN adduser -D -u 10001 aibreak && mkdir -p /data && chown aibreak:aibreak /data
COPY --from=build /out/aibreakd /usr/local/bin/aibreakd
ENV AIBREAK_ADDR=:8080 \
    AIBREAK_DB=/data/aibreak.db
EXPOSE 8080
VOLUME ["/data"]
USER aibreak
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["aibreakd"]
