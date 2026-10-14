FROM golang:1.26.4-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS ca-certs

FROM debian:trixie-slim@sha256:a29215f6a35e51e22adffa17f89e9d2ef06214e64a2bad10d765c46aea49f11f

COPY --from=ca-certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

ENV DEBIAN_FRONTEND=noninteractive

RUN sed -i 's#http://deb.debian.org#https://deb.debian.org#' /etc/apt/sources.list.d/debian.sources \
    && apt-get -o Acquire::https::CAInfo=/etc/ssl/certs/ca-certificates.crt update \
    && apt-get -o Acquire::https::CAInfo=/etc/ssl/certs/ca-certificates.crt install -y --no-install-recommends ca-certificates curl nginx openssl systemd systemd-sysv \
    && rm -rf /var/lib/apt/lists/*

STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
