# Build stage
FROM golang:1.23.5 AS builder
WORKDIR /app

# Install protoc and required packages
RUN apt-get update && apt-get install -y protobuf-compiler
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

# Set PATH to include Go binaries
ENV PATH="$PATH:$(go env GOPATH)/bin"

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Generate proto files
RUN protoc --go_out=. --go-grpc_out=. proto/order.proto
RUN CGO_ENABLED=0 GOOS=linux go build -o service2 .

# Run stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/service2 .
EXPOSE 50052
CMD ["./service2"]
