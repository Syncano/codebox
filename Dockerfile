FROM golang:1.11

COPY ./Gopkg.* ./Makefile /go/src/github.com/Syncano/codebox/
WORKDIR /go/src/github.com/Syncano/codebox

RUN set -ex \
    && apt-get update \
    && apt-get -y install squashfs-tools \
    && curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

RUN make testdeps
COPY . /go/src/github.com/Syncano/codebox
