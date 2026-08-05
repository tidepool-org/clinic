# Development
FROM golang:1.24.3-alpine AS development
WORKDIR /go/src/github.com/tidepool-org/clinic
RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk --no-cache add make ca-certificates tzdata && \
    adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic
USER tidepool
RUN --mount=type=cache,target=/go/pkg/mod go install github.com/air-verse/air@v1.61.7
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["air"]

# Production
FROM golang:1.24.3-alpine AS production
WORKDIR /go/src/github.com/tidepool-org/clinic
RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk --no-cache add ca-certificates tzdata && \
    adduser -D tidepool
WORKDIR /go/src/github.com/tidepool-org/clinic
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/home/tidepool/.cache/go-build \
    ./build.sh
USER tidepool
WORKDIR /go/src/github.com/tidepool-org/clinic/dist
CMD ["./clinic"]
