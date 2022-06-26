FROM golang:1.18-bullseye as build
WORKDIR /go/src/app
ADD . /go/src/app
RUN go get -d -v ./...
RUN GOOS=linux GOARCH=amd64 go build -o /go/bin/app /go/src/app/cmd
ENTRYPOINT ["/go/bin/app"]
