FROM golang:1.21 AS builder

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Use minimal base image for smaller size
FROM gcr.io/distroless/static-debian11

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main .

# Use ENTRYPOINT for proper signal handling
ENTRYPOINT ["/app/main"]
