FROM golang:1.21.1-bullseye as build
WORKDIR /app
COPY server.go go.mod go.sum ./
RUN go mod tidy \
 && CGO_ENABLED=0 GOOS=linux go build -o test-http-server

FROM debian:bullseye-slim
WORKDIR /app
COPY --from=build /app/test-http-server ./
EXPOSE 8080
ENTRYPOINT ["/app/test-http-server"]
