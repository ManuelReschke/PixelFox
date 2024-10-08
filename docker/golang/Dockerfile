# Build stage
FROM golang:1.23-alpine AS fetch-stage
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Generate
FROM ghcr.io/a-h/templ:latest AS generate-stage
COPY --chown=65532:65532 . /app
WORKDIR /app
RUN ["templ", "generate"]

FROM golang:1.23-alpine AS build-stage
COPY --from=generate-stage /app /app
WORKDIR /app
RUN go build -o /app/pixelfox-app

# Run stage
FROM alpine:latest
WORKDIR /root/
COPY --from=build-stage /app/. .

EXPOSE 8080
CMD ["./pixelfox-app"]