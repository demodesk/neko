#
# Stage 1: Build.
#
FROM golang:1.20-bullseye as build
WORKDIR /src

COPY . .

ENV CGO_ENABLED=0
RUN go build -o telemetry_server -ldflags "-s -w" main.go

#
# Stage 2: Run.
#
FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /src/telemetry_server /usr/bin/telemetry_server

ENTRYPOINT ["/usr/bin/telemetry_server"]
