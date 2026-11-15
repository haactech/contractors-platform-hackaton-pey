# Go API image for Cloudflare Containers.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
# Writable directory for the SQLite file (distroless has no shell to create it).
RUN mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /api
COPY --from=build --chown=nonroot:nonroot /out/data /data
# SQLite lives on the container disk: it is reset when the container sleeps,
# restarts or is redeployed. Good enough for demos and e2e tests.
ENV HOST=0.0.0.0 PORT=8080 DATABASE_PATH=/data/app.db CLIENTS_DATABASE_PATH=/data/clients.db PAYMENTS_DATABASE_PATH=/data/payments.db
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
