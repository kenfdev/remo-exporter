FROM golang:1.11.5 as build
LABEL maintainer "kenfdev@gmail.com"


COPY ./ /go/src/github.com/kenfdev/remo-exporter
WORKDIR /go/src/github.com/kenfdev/remo-exporter

RUN go get \
     && go test ./... \
     && go build -o /bin/main

# Run a gofmt and exclude all vendored code.
RUN test -z "$(gofmt -l $(find . -type f -name '*.go' -not -path "./vendor/*"))" \
     && go test $(go list ./... | grep -v integration | grep -v /vendor/ | grep -v /template/) -cover \
     && CGO_ENABLED=0 GOOS=linux go build -a -o remo-exporter .

FROM alpine:3.9

RUN apk --no-cache add ca-certificates \
     && addgroup exporter \
     && adduser -S -G exporter exporter
USER exporter
COPY --from=build /go/src/github.com/kenfdev/remo-exporter/remo-exporter .
ENV LISTEN_PORT=9352
EXPOSE 9352
ENTRYPOINT [ "./remo-exporter" ]