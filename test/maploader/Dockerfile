FROM golang:1.20

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

COPY *.go ./

RUN go build -o /maploader