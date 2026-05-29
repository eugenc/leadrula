# Build
FROM golang:1.25.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -o /out/bootstrap ./cmd/bootstrap \
 && CGO_ENABLED=0 go build -o /out/seed-demo ./cmd/seed-demo

# Run
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/bootstrap /app/bootstrap
COPY --from=build /out/seed-demo /app/seed-demo
EXPOSE 8080
CMD ["/app/server"]
