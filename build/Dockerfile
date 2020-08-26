FROM alpine:3.11

ARG UID=1000
ARG GID=1000

ENV SQUASHFUSE_VERSION 0.1.103
ENV WRAPPERPATH /usr/local/bin/codewrapper

RUN set -ex \
    \
    # Install squashfuse
    && apk --no-cache add --virtual .build-deps \
        build-base \
        autoconf \
        automake \
        libtool \
        xz-dev \
        lz4-dev \
        lzo-dev \
        zlib-dev \
        fuse-dev \
    && wget https://github.com/vasi/squashfuse/archive/${SQUASHFUSE_VERSION}.tar.gz -O squashfuse.tar.gz \
    && tar zxf squashfuse.tar.gz -C / \
    && cd /squashfuse-${SQUASHFUSE_VERSION} \
    && ./autogen.sh \
    && ./configure \
    && make install \
    && rm -rf /squashfuse-${SQUASHFUSE_VERSION} \
    && apk del .build-deps \
    \
    # Install requirements
    && addgroup -S -g $GID codebox \
    && adduser -S -D -G codebox -u $UID codebox \
    && apk --no-cache add \
        lzo \
        lz4-libs \
        xz-libs \
        fuse \
        ca-certificates \
        # tini to avoid zombies and to properly process squashfuse umounts
        tini

COPY codebox /usr/local/bin/codebox
COPY codewrapper ${WRAPPERPATH}

WORKDIR /home/codebox
USER codebox
EXPOSE 9000
CMD ["codebox"]
ENTRYPOINT ["/sbin/tini", "--"]
