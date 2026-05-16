FROM golang:1.26-alpine AS build

WORKDIR /build

ARG CGO_ENABLED=0

ENV CGO_ENABLED=${CGO_ENABLED} \
    GOCACHE=/root/.cache/go-build \
    GOMODCACHE=/go/pkg/mod

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -tags netgo,osusergo \
    ./cmd/main


FROM scratch AS release

WORKDIR /app

COPY --from=build /build/main /main

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZDIR=/usr/share/zoneinfo
ENV TZ=Europe/Moscow

USER 65532:65532

ENTRYPOINT ["/main"]
