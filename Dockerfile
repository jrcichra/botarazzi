FROM golang:1.15.3-alpine3.12 as builder
WORKDIR /app
COPY . .
RUN go build
FROM alpine:3.12
WORKDIR     /app
COPY --from=builder /app/botarazzi .
RUN apk add ffmpeg
CMD ./botarazzi