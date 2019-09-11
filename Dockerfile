FROM golang:1.12 AS build

COPY . /go/src/github.com/piersharding/dask-operator
WORKDIR /go/src/github.com/piersharding/dask-operator
RUN GO111MODULE=on go build -o /go/bin/dask-operator

FROM debian:stretch-slim

COPY --from=build /go/bin/dask-operator /usr/bin/dask-operator
