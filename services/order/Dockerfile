FROM golang:1.26.0-alpine AS builder

RUN apk add --no-cache build-base

WORKDIR /app
COPY common/go.mod common/go.sum ./common/
COPY services/order/go.mod services/order/go.sum ./services/order/
RUN go work init && go work use ./services/order ./common

WORKDIR /app/common
RUN go mod download

WORKDIR /app/services/order
RUN go mod download

WORKDIR /app
COPY common/ common/
COPY services/order/ services/order/

RUN go work sync

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/order ./services/order/cmd/main.go

FROM alpine:latest
WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/order .

CMD ["./order"]