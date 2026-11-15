# syntax=docker/dockerfile:1
# ---- build stage ----
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/cache22 ./cmd/cache22 && mkdir -p /data && chown 65532:65532 /data

# ---- runtime stage: distroless, non-root ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/cache22 /cache22
COPY --from=build --chown=65532:65532 /data /data
VOLUME ["/data"]
EXPOSE 8080
USER nonroot:nonroot
ENV CACHE22_ADDR=:8080 CACHE22_DATA=/data/cache22.db
ENTRYPOINT ["/cache22"]
