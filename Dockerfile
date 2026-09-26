FROM golang:1.27.1 AS builder

WORKDIR /src

# Every dependency is public, so no GOPRIVATE and no credential mount.
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 go build -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o /dist/mmdb .

# Distroless rather than a full base: this is one static binary that only reads
# and writes files.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /dist/mmdb /usr/local/bin/mmdb

ENTRYPOINT ["/usr/local/bin/mmdb"]
