FROM golang:1.27 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /main .

FROM alpine:3.24
RUN apk add --no-cache ca-certificates
COPY --from=builder /main /main
ENTRYPOINT ["/main"]
