FROM golang:1.24.2 AS build
WORKDIR /app
COPY server.go go.mod go.sum ./
RUN go mod download \
 && CGO_ENABLED=0 GOOS=linux go build -o test-http-server

# hadolint ignore=DL3007
FROM gcr.io/distroless/static-debian11:latest
WORKDIR /
COPY --from=build /app/test-http-server ./
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/test-http-server"]
