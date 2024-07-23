# syntax=docker/dockerfile:1

FROM golang:1.17-alpine

WORKDIR /app
COPY . /app

RUN go build -v -o ./bin/ ./cmd/tgp

EXPOSE 8080

CMD ["sh" , "-c", "./bin/tgp -P 8080 -H 0.0.0.0 \"$TGPROXY_CHANNEL\""]
