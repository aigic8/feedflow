FROM golang:1.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o ./main ./cmd/feedflow

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/main ./main

CMD ["./main"]
