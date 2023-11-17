FROM golang:1.21 as builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build

FROM debian:12-slim
LABEL org.opencontainers.image.source https://github.com/sikalabs/prometheus-node-exporter-to-json
COPY \
  --from=builder \
  /build/prometheus-node-exporter-to-json \
  /usr/local/bin/prometheus-node-exporter-to-json
CMD ["/usr/local/bin/prometheus-node-exporter-to-json"]
EXPOSE 8000
