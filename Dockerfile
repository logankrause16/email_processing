FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o catchall ./cmd/server

# Create a minimal image
FROM alpine:3.18

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/catchall .
COPY --from=builder /app/configs /app/configs

# Expose the application port
EXPOSE 8080

# Set the entrypoint
ENTRYPOINT ["/app/catchall"]

# Default command
CMD ["-processor", "-workers=8"]