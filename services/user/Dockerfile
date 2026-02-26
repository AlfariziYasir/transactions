FROM golang:1.26.0-alpine AS builder

RUN apk add --no-cache build-base

WORKDIR /app
COPY common/go.mod common/go.sum ./common/
COPY services/user/go.mod services/user/go.sum ./services/user/
RUN go work init && go work use ./services/user ./common

WORKDIR /app/common
RUN go mod download

WORKDIR /app/services/user
RUN go mod download

WORKDIR /app
COPY common/ common/
COPY services/user/ services/user/

RUN go work sync

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/user ./services/user/cmd/main.go

FROM alpine:latest
WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/user .

CMD ["./user"]