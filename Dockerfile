FROM golang:1.25.0-alpine as builder
WORKDIR /app
COPY . .
RUN go build
FROM alpine
WORKDIR /app
EXPOSE 8080
COPY --from=builder /app/botarazzi .
COPY --from=builder /app/welcome.ogg .
RUN apk add --no-cache ffmpeg
CMD ./botarazzi
