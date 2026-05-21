FROM golang:1.26-bookworm AS api-builder

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/twins-api ./cmd/twins-api

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl \
  && rm -rf /var/lib/apt/lists/* \
  && useradd --system --create-home --home-dir /app twins
WORKDIR /app
COPY --from=api-builder /out/twins-api /usr/local/bin/twins-api
RUN mkdir -p /data && chown -R twins:twins /data /app

USER twins
ENV TWINS_HTTP_ADDR=:8080
ENV TWINS_DATA_PATH=/data/twins-store.json
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD curl -fsS http://127.0.0.1:8080/readyz >/dev/null || exit 1

CMD ["/usr/local/bin/twins-api"]
