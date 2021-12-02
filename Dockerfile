FROM golang:1.17.3-buster as build
WORKDIR /app
COPY server.go go.mod go.sum .
RUN CGO_ENABLED=0 GOOS=linux go build -o test-http-server

FROM debian:buster-slim
WORKDIR /app
COPY --from=build /app/test-http-server .
EXPOSE 8080
ENTRYPOINT ["/app/test-http-server"]
