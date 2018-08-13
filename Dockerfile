FROM golang:1.8

RUN go get github.com/go-redis/redis
RUN go get github.com/gocql/gocql
RUN go get github.com/gorilla/mux

EXPOSE 8080

RUN mkdir -p /src/rofael.net/session
WORKDIR /src/rofael.net/session
COPY .\ .