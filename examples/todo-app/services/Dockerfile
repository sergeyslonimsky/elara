# syntax=docker/dockerfile:1.7
#
# Builds one of the toy todo-app services. Which one is selected by the SERVICE
# build arg (api | worker | notifier | seed), matching a package under ./cmd.
FROM golang:1.26.5-alpine3.24 AS build

ARG SERVICE=api

WORKDIR /app

COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/service ./cmd/${SERVICE}

FROM scratch
COPY --from=build /bin/service /bin/service
ENTRYPOINT ["/bin/service"]
