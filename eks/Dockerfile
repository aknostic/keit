# Start with the official Golang image as the build environment
FROM golang:1.22 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files
COPY go.mod go.sum ./

# Download and cache the Go modules
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go app for Linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /main .

# Start a new stage from scratch
FROM alpine:3.18

# Install certificates for HTTPS support
RUN apk add --no-cache ca-certificates

# Copy the binary from the builder
COPY --from=builder /main /main

# Set the binary as the entrypoint
ENTRYPOINT ["/main"]
