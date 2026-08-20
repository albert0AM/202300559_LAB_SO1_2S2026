# Etapa 1: compilar el binario
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY main.go ./
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o api main.go

# Etapa 2: imagen final, mínima
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/api .
EXPOSE 8080
ENTRYPOINT ["./api"]
