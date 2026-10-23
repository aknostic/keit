# Start with the official Golang image as the build environment
# Go 1.27.3+ distrusts the compromised eMudhra intermediates (CVE-2026-111943)
FROM golang:1.27.3 AS builder

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
FROM alpine:3.24

# Install certificates for HTTPS support; upgrade both packages so the base
# image's ca-certificates-bundle doesn't keep the old (pre-eMudhra-fix) roots
RUN apk add --no-cache --upgrade ca-certificates ca-certificates-bundle

# Copy the binary from the builder
COPY --from=builder /main /main

# Set the binary as the entrypoint
ENTRYPOINT ["/main"]
