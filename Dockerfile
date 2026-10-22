FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod ./
COPY go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o main ./cmd/api

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

# config.yaml holds secrets, so it is never part of the image: mount it at run time, e.g.
#   docker run -v "$PWD/config.yaml:/app/config.yaml:ro" -p 3000:3000 backend
# (or set APP_* environment variables)
USER app

# Expose the application port
EXPOSE 3000

# Command to run the executable
CMD ["./main"]
