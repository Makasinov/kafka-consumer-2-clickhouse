FROM golang:1.13-alpine3.10 as builder

RUN apk update
RUN apk add ca-certificates curl bash git
RUN apk add g++ gcc pkgconf make zlib-dev
RUN apk add zstd-dev

WORKDIR /root
RUN git clone https://github.com/edenhill/librdkafka.git
WORKDIR /root/librdkafka
RUN echo STATIC_LIB_libzstd
RUN STATIC_LIB_libzstd=/usr/lib/libzstd.a ./configure --reconfigure --prefix /usr --enable-static
RUN make
RUN make install

WORKDIR /go/src/kafka-consumer

# cache dependencies
ADD go.* ./
RUN go mod download

ADD . ./
RUN go build -o kafka-consumer cmd/kafka-consumer/*.go
RUN mv kafka-consumer /usr/bin/kafka-consumer

# download clickhouse packages
RUN apk add make
RUN make prepare

# ----------------------------------------------------------------------------------------------------------------------
FROM debian:9 as ClickHouse-Installer

COPY --from=builder /go/src/kafka-consumer/resources/* /resources/
RUN dpkg -i /resources/clickhouse-common-static.deb
RUN dpkg -i /resources/clickhouse-server-base.deb
RUN dpkg -i /resources/clickhouse-common-dbg.deb
RUN dpkg -i /resources/clickhouse-client.deb

COPY --from=builder /usr/bin/kafka-consumer /usr/bin/
COPY --from=builder /usr/local/lib/librdkafka.so.1 /usr/local/lib/
COPY --from=builder /lib/ld-musl-x86_64.so.1 /lib/
COPY --from=builder /usr/lib/libzstd.so.1 /usr/lib/
COPY --from=builder /lib/libz.so.1 /lib/

COPY --from=builder /go/src/kafka-consumer/config/* /etc/kafka-consumer/conf.d/

ENTRYPOINT [ "/usr/bin/kafka-consumer" ]
