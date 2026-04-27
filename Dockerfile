# ── Stage 1: build frontend ────────────────────────────────────────────────
FROM node:22-alpine AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# ── Stage 2: build Go binary ───────────────────────────────────────────────
FROM golang:1.25-alpine AS go-builder

WORKDIR /app

# Cache module downloads separately from source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o server ./cmd/server

# ── Stage 3: final minimal image ──────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app

# Binary
COPY --from=go-builder  /app/server          ./server

# Migration SQL files (read at runtime by golang-migrate)
COPY --from=go-builder  /app/migrations      ./migrations

# Built frontend assets
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

EXPOSE 8080

CMD ["./server"]
