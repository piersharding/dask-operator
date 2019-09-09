FROM golang:1.12 AS build

COPY . /go/src/dask-controller
WORKDIR /go/src/dask-controller
RUN go get -u github.com/golang/dep/cmd/dep && dep ensure && go build -o /go/bin/dask-controller

FROM debian:stretch-slim

COPY --from=build /go/bin/dask-controller /usr/bin/dask-controller
