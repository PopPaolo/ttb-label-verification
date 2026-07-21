# Stage 1: frontend build
FROM node:24-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: backend build
FROM golang:1.26-alpine AS backend
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# Stage 3: runtime
FROM alpine:3.21
RUN adduser -D -u 10001 app
USER app
WORKDIR /app
COPY --from=backend /server ./server
COPY --from=frontend /app/web/dist ./web/dist
ENV ADDR=:8080 STATIC_DIR=/app/web/dist
EXPOSE 8080
ENTRYPOINT ["/app/server"]
