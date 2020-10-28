FROM golang:1.15.3 as builder
WORKDIR /app
COPY . .
RUN go build
FROM debian
WORKDIR     /app
COPY --from=builder /app/botarazzi .
RUN apt update && apt install -y ffmpeg ca-certificates
CMD ./botarazzi
