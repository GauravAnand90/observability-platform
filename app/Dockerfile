# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src

# `go mod tidy` resolves the OpenTelemetry modules into go.sum if it isn't
# committed yet. Commit go.sum (make tidy) to make builds reproducible.
COPY . .
RUN go mod tidy && go mod download

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/app .

# Distroless: no shell, no package manager, runs as non-root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
