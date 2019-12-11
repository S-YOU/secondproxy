FROM golang:1.13-alpine as builder

WORKDIR /go/src/github.com/s-you/secondproxy

ADD . .

RUN GO111MODULE=off CGO_ENABLED=0 go build -a -ldflags "-extldflags '-static' -s -w" -gcflags="-trimpath=${PWD}" -o /app

FROM alpine:3.10

WORKDIR /

COPY --from=builder /app /

CMD /app
