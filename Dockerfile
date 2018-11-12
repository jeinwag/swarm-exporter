FROM golang:1.9
WORKDIR /go/src/github.com/jeinwag/swarm-exporter/
ADD . .

RUN CGO_ENABLED=0 go build -o swarm-exporter

FROM scratch
EXPOSE 9515

COPY --from=0 /go/src/github.com/jeinwag/swarm-exporter/swarm-exporter /bin/swarm-exporter
ENTRYPOINT ["/bin/swarm-exporter"]
