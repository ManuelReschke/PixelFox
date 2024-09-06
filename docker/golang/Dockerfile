# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -o /app/pixelfox-app

# Run stage
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/. .

#COPY .env .env

EXPOSE 8080

CMD ["./pixelfox-app"]