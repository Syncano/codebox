FROM golang:1.11
ENV SQUASHFUSE_VERSION 0.1.103

RUN set -ex \
    && apt-get update \
    \
    # Install squashfuse
    && apt-get --no-install-recommends -y install \
        autoconf \
        automake \
        libtool \
        liblzma-dev \
        liblz4-dev \
        liblzo2-dev \
        zlib1g-dev \
        libfuse-dev \
    && wget https://github.com/vasi/squashfuse/archive/${SQUASHFUSE_VERSION}.tar.gz -O squashfuse.tar.gz \
    && tar zxf squashfuse.tar.gz -C / \
    && cd /squashfuse-${SQUASHFUSE_VERSION} \
    && ./autogen.sh \
    && ./configure \
    && make install \
    && ldconfig \
    && cd .. \
    && rm -rf /squashfuse-${SQUASHFUSE_VERSION} \
    \
    # Install squashfs tools and fuse
    && apt-get --no-install-recommends -y install \
        squashfs-tools \
        fuse \
    && apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY ./Gopkg.* ./Makefile /go/src/github.com/Syncano/codebox/
WORKDIR /go/src/github.com/Syncano/codebox
RUN make testdeps
