# Railway builds from the repo root; backend sources live in backend/.
FROM golang:1.25.1-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -o /out/bootstrap ./cmd/bootstrap \
 && CGO_ENABLED=0 go build -o /out/bootstrap-platform ./cmd/bootstrap-platform \
 && CGO_ENABLED=0 go build -o /out/seed-demo ./cmd/seed-demo

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/bootstrap /app/bootstrap
COPY --from=build /out/bootstrap-platform /app/bootstrap-platform
COPY --from=build /out/seed-demo /app/seed-demo
EXPOSE 8080
CMD ["/app/server"]
