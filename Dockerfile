# Development
FROM golang:1.21.6-alpine AS development
WORKDIR /go/src/github.com/tidepool-org/clinic
RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk --no-cache add make ca-certificates tzdata && \
    adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic
USER tidepool
RUN go install github.com/cosmtrek/air@latest
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["air"]

# Production
FROM golang:1.21.6-alpine AS production
WORKDIR /go/src/github.com/tidepool-org/clinic
RUN apk --no-cache update && \
    apk --no-cache upgrade && \
    apk --no-cache add ca-certificates tzdata && \
    adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic
USER tidepool
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["./dist/clinic"]

