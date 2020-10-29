FROM golang:1.15.3-alpine as builder
WORKDIR /app
COPY . .
RUN go build
FROM alpine
WORKDIR /app
COPY --from=builder /app/botarazzi .
COPY --from=builder /app/welcome.ogg .
RUN apk add --no-cache ffmpeg
CMD ./botarazzi
