FROM golang:1.15.3-alpine as builder
WORKDIR /app
COPY . .
RUN go build
FROM sjourdan/ffprobe
WORKDIR /app
COPY --from=builder /app/botarazzi .
RUN apk add ca-certificates
CMD ./botarazzi
