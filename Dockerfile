FROM golang:1.27-alpine AS build

ARG SERVICE
ARG PORT
ARG VERSION=dev


WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -v \
    -ldflags "-X github.com/faraquic/lotty-ab-platform/pkg/config.ServiceVersion=${VERSION}" \
    -o /bin/service \
    ./services/${SERVICE}

FROM alpine:3

COPY --from=build /bin/service /bin/service

EXPOSE ${PORT}

ENTRYPOINT ["/bin/service"]
