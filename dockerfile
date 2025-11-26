FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o taskapi .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/taskapi .
EXPOSE 8080
CMD ["./taskapi"]