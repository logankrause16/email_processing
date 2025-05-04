# Docker file for the application. It is mainly used for building the docker-compose images and containers

# had to look up and play with this for awhile until I got it right. I was not explicitly running the 
# go mod download command after copying the go.mod and go.sum files. I was just copying the entire directory and
# then running the go build command like you would in the Node stuff I'm used to.

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