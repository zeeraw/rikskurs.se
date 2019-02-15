FROM golang:1.11

WORKDIR /go/src/github.com/zeeraw/rikskurs.se
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

EXPOSE 8080

CMD ["rikskurs.se"]
