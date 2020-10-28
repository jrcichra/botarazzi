FROM golang:1.15.3 as builder
WORKDIR /app
COPY . .
RUN go build
FROM debian
WORKDIR     /app
COPY --from=builder /app/botarazzi .
RUN apk add ffmpeg ffprobe ffplay
CMD ./botarazzi
