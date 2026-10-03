ARG BASE_IMAGE=alpine:3.24.2
FROM ${BASE_IMAGE}

ARG EXPORTER_BINARY="remo-exporter-linux-amd64"
COPY ${EXPORTER_BINARY} /usr/local/bin/remo-exporter

RUN apk --no-cache add ca-certificates \
     && addgroup exporter \
     && adduser -S -G exporter exporter
USER exporter

ENV PORT=9352
EXPOSE 9352
ENTRYPOINT [ "/usr/local/bin/remo-exporter" ]
