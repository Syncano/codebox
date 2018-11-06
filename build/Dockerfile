FROM alpine:3.7

RUN set -ex \
    && addgroup -S codebox \
    && adduser -S -G codebox codebox \
    && apk --no-cache add \
        zlib \
        lzo \
        lz4 \
        xz \
        fuse \
        ca-certificates

COPY codebox-static /usr/local/bin/codebox

WORKDIR /home/codebox
USER codebox
EXPOSE 9000
