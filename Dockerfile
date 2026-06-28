# Build frontend
FROM node:24-alpine AS frontend-builder
RUN corepack enable
WORKDIR /web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# Build backend
FROM golang:1.26-alpine AS backend-builder
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Copy frontend build artifacts to expected location for embedding
COPY --from=frontend-builder /web/dist ./cmd/server/dist

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o main ./cmd/server

# Final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=backend-builder /app/main .

ENV SERVER_PORT=8080 \
    SERVER_MODE=release \
    DATABASE_TYPE=sqlite \
    DATABASE_PATH=/data/music.db \
    SERVER_UPLOAD_DIR=/data/uploads

VOLUME ["/data"]
EXPOSE 8080

CMD ["./main"]
