# Development
FROM golang:1.19-alpine AS development
WORKDIR /go/src/github.com/tidepool-org/clinic
RUN adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic
USER tidepool
RUN go install github.com/cosmtrek/air@latest
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["air"]

# Production
FROM golang:1.19-alpine AS production
WORKDIR /go/src/github.com/tidepool-org/clinic
RUN adduser -D tidepool && \
    chown -R tidepool /go/src/github.com/tidepool-org/clinic
USER tidepool
COPY --chown=tidepool . .
RUN ./build.sh
CMD ["./dist/clinic"]
