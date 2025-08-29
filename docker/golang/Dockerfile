# Build stage
FROM golang:1.25-alpine AS fetch-stage
WORKDIR /app
RUN apk add --no-cache build-base ffmpeg libwebp-dev
# enable CGO (needed for kolesa-teamgo webp)
ENV CGO_ENABLED=1
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Generate
FROM ghcr.io/a-h/templ:latest AS generate-stage
COPY --chown=65532:65532 . /app
WORKDIR /app
RUN ["templ", "generate"]

FROM golang:1.25-alpine AS build-stage
COPY --from=generate-stage /app /app
WORKDIR /app
RUN go build -o /app/pixelfox-app ./cmd/pixelfox

# Run stage
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ffmpeg libwebp-dev
ENV CGO_ENABLED=1
COPY --from=build-stage /app/. .

EXPOSE 8080
CMD ["./pixelfox-app"]