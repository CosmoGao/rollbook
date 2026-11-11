FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /rollbook ./cmd/rollbook

FROM alpine:3.21
RUN addgroup -S rollbook && adduser -S -G rollbook rollbook && mkdir /data && chown rollbook:rollbook /data
COPY --from=build /rollbook /usr/local/bin/rollbook
USER rollbook
ENV ROLLBOOK_DB_PATH=/data/rollbook.db ROLLBOOK_LISTEN_ADDR=:8080
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s CMD wget -q -O - http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/rollbook", "serve"]
