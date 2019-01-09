FROM  quay.io/prometheus/busybox:latest
LABEL maintainer="The Prometheus Authors <prometheus-developers@googlegroups.com>"

COPY zuul_exporter /bin/zuul_exporter

EXPOSE      9532 9532 9532/udp
ENTRYPOINT  [ "/bin/zuul_exporter" ]
