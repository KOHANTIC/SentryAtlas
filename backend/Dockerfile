FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /bin/server ./cmd/server

FROM alpine:3.24

RUN apk add --no-cache ca-certificates wget \
    && addgroup -S app && adduser -S -G app app

COPY --from=build /bin/server /bin/server

# The instance has 0.5 GB; the Go runtime collects harder near this limit
# instead of letting the platform kill the process.
ENV GOMEMLIMIT=400MiB

USER app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/bin/server"]
