FROM golang:1.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build ./cmd/feedflow -o main

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/main .

CMD ["./main"]
