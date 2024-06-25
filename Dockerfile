# Development
FROM golang:1.22.4-alpine AS development
WORKDIR /go/src/github.com/tidepool-org/clinic
RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk --no-cache add make ca-certificates tzdata && \
    adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic
USER tidepool
RUN go install github.com/air-verse/air@v1.52.2
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["air"]

# Production
FROM golang:1.22.4-alpine AS production
WORKDIR /go/src/github.com/tidepool-org/clinic
RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk --no-cache add ca-certificates tzdata && \
    adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic
USER tidepool
COPY --chown=tidepool . .
RUN ./build.sh
WORKDIR /go/src/github.com/tidepool-org/clinic/dist
CMD ["./clinic"]

