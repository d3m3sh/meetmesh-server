# BUILD
FROM golang:latest AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY ./main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o signaling .

# PROD
FROM alpine:latest

WORKDIR /root/
COPY --from=builder /app/signaling .
EXPOSE ${PORT:-8080}

CMD ["./signaling"]