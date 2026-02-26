FROM golang:1.26.0-alpine AS builder

RUN apk add --no-cache build-base

WORKDIR /app
COPY common/go.mod common/go.sum ./common/
COPY services/inventory/go.mod services/inventory/go.sum ./services/inventory/
RUN go work init && go work use ./services/inventory ./common

WORKDIR /app/common
RUN go mod download

WORKDIR /app/services/inventory
RUN go mod download

WORKDIR /app
COPY common/ common/
COPY services/inventory/ services/inventory/

RUN go work sync

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/inventory ./services/inventory/cmd/main.go

FROM alpine:latest
WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/inventory .

CMD ["./inventory"]