FROM golang:1.16-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
COPY home.html ./
RUN go mod download

COPY *.go ./

RUN go build -o /afreeca-server

# EXPOSE 8080

CMD [ "/afreeca-server" ]
