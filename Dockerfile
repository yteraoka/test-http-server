FROM golang:1.14.0-alpine3.11 as build
WORKDIR /app
COPY server.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o test-http-server

FROM alpine:3.11
WORKDIR /app
COPY --from=build /app/test-http-server .
EXPOSE 8080
ENTRYPOINT ["/app/test-http-server"]
