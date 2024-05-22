ARG BASE_IMAGE=alpine:3.20
FROM ${BASE_IMAGE}

ARG EXPORTER_BINARY="remo-exporter-linux-amd64"
COPY ${EXPORTER_BINARY} ./remo-exporter

RUN apk --no-cache add ca-certificates \
     && addgroup exporter \
     && adduser -S -G exporter exporter
USER exporter

ENV LISTEN_PORT=9352
EXPOSE 9352
ENTRYPOINT [ "./remo-exporter" ]