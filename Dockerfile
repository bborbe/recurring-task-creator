ARG SERVICE_NAME=recurring-task-creator

FROM golang:1.26.4 AS build
WORKDIR /workspace
ARG SERVICE_NAME
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=bind,target=. \
    GOCACHE=/root/.cache/go-build \
    GOMODCACHE=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w" \
    -installsuffix cgo \
    -o /${SERVICE_NAME} \
    ./cmd/${SERVICE_NAME}
CMD ["/bin/bash"]

FROM alpine:3.24 AS alpine
RUN apk --no-cache add ca-certificates

FROM scratch
ARG SERVICE_NAME
ARG BUILD_GIT_VERSION=dev
ARG BUILD_GIT_COMMIT=none
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="${SERVICE_NAME}"
LABEL org.opencontainers.image.description="Go microservice recurring-task-creator/${SERVICE_NAME}"
LABEL org.opencontainers.image.vendor="Benjamin Borbe"
LABEL org.opencontainers.image.licenses="BSD-2-Clause"
LABEL org.opencontainers.image.source="https://github.com/bborbe/recurring-task-creator"
LABEL org.opencontainers.image.documentation="https://github.com/bborbe/recurring-task-creator"
LABEL org.opencontainers.image.version="${BUILD_GIT_VERSION}"
LABEL org.opencontainers.image.created="${BUILD_DATE}"
LABEL org.opencontainers.image.revision="${BUILD_GIT_COMMIT}"

COPY --from=build /${SERVICE_NAME} /${SERVICE_NAME}
COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /usr/local/go/lib/time/zoneinfo.zip /
ENV ZONEINFO=/zoneinfo.zip
ENV BUILD_GIT_VERSION=${BUILD_GIT_VERSION}
ENV BUILD_GIT_COMMIT=${BUILD_GIT_COMMIT}
ENV BUILD_DATE=${BUILD_DATE}
ENTRYPOINT ["/${SERVICE_NAME}"]
