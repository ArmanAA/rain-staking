FROM golang:1.24-alpine AS builder

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /stakingd ./cmd/stakingd

# Runtime image
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /stakingd /stakingd
COPY migrations/ /migrations/

EXPOSE 8080 9090

ENTRYPOINT ["/stakingd"]
