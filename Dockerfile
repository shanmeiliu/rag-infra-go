FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/rag-api ./cmd/api

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/rag-api /app/rag-api

RUN mkdir -p /app/uploads


ARG APP_PORT=8080
ENV PORT=${APP_PORT}

EXPOSE ${APP_PORT}

CMD ["/app/rag-api"]