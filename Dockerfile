# builds on the machine's own CPU and cross-compiles for the target one, so a multi-arch image is fast
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy go mod and sum files
COPY go.mod ./
COPY go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o main ./cmd/api

# Use a smaller image for the final stage
FROM alpine:3.20

# Set timezone
ENV TZ=Asia/Vientiane
# Install tzdata for timezone support
RUN apk add --no-cache tzdata

RUN apk --no-cache add ca-certificates

# Run as a non-root user
RUN adduser -D -H app
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/main .

# config.yaml is baked into the image, so keep the image private (never push it to a public repo).
# A config.yaml mounted at /app/config.yaml at run time still overrides this one.
COPY config.yaml .
USER app

# Expose the application port
EXPOSE 3000

# Command to run the executable
CMD ["./main"]
