# Build frontend
FROM node:18-alpine AS frontend-builder
WORKDIR /web
COPY web/package*.json ./
RUN npm install
COPY web/ .
RUN npm run build

# Build backend
FROM golang:1.24-alpine AS backend-builder
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

WORKDIR /root/

COPY config.yaml ./
COPY --from=backend-builder /app/main .

EXPOSE 8080

CMD ["./main"]
