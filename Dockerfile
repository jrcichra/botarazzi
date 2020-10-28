FROM golang:1.15.3 as builder
WORKDIR /app
COPY . .
RUN go build
FROM debian
WORKDIR     /app
COPY --from=builder /app/botarazzi .
RUN sudo apt update && sudo apt install  ffmpeg ffprobe ffplay
CMD ./botarazzi
