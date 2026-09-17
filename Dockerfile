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

FROM alpine:3

ARG PORT
ENV PORT=${PORT}

COPY --from=build /bin/service /bin/service

EXPOSE ${PORT}

ENTRYPOINT ["/bin/service"]
