# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o wiredrop .

# Production scratch stage (zero overhead, ~7MB total image)
FROM scratch
COPY --from=builder /app/wiredrop /wiredrop
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/wiredrop"]
