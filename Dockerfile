# Single image: the Go API also serves the built Mini App (used by Railway and docker compose).
FROM node:22-alpine AS web
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.25-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api .

FROM alpine:3.20
# Root CAs come from the build stage and tzdata is embedded in the binary, so this layer
# needs no package downloads.
RUN adduser -D -u 10001 app
COPY --from=api /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=api /out/api /usr/local/bin/api
COPY --from=web /src/dist /srv/web
USER app
ENV GIN_MODE=release STATIC_DIR=/srv/web PORT=8080
EXPOSE 8080
CMD ["api"]
