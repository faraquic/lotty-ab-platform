FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,id=labp-go-mod,target=/go/pkg/mod \
    go mod download

COPY pkg/ ./pkg/
ARG SERVICE
COPY services/${SERVICE}/ ./services/${SERVICE}/
ARG VERSION=dev

RUN --mount=type=cache,id=labp-go-mod,target=/go/pkg/mod \
    --mount=type=cache,id=labp-go-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -buildvcs=false \
    -ldflags "-s -w -X github.com/faraquic/lotty-ab-platform/pkg/config.ServiceVersion=${VERSION}" \
    -o /bin/service \
    ./services/${SERVICE}

FROM alpine:3.22

ARG PORT
RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
COPY --from=build --chown=app:app /bin/service /usr/local/bin/service

USER app
EXPOSE ${PORT}

ENTRYPOINT ["/usr/local/bin/service"]
