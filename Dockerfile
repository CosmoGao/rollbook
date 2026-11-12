FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS build
ARG TARGETOS
ARG TARGETARCH
ARG ROLLBOOK_VERSION=dev
ARG ROLLBOOK_COMMIT=
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X github.com/CosmoGao/rollbook/internal/buildinfo.Version=$ROLLBOOK_VERSION -X github.com/CosmoGao/rollbook/internal/buildinfo.Commit=$ROLLBOOK_COMMIT" -o /rollbook ./cmd/rollbook

FROM alpine:3.21
RUN addgroup -S -g 10001 rollbook && adduser -S -D -H -u 10001 -G rollbook rollbook \
    && mkdir /data && chown 10001:10001 /data
COPY --from=build /rollbook /usr/local/bin/rollbook
USER 10001:10001
ENV ROLLBOOK_DB_PATH=/data/rollbook.db ROLLBOOK_LISTEN_ADDR=:8080
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s CMD wget -q -O - http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/rollbook", "serve"]
