FROM golang:1.22 AS build
WORKDIR /src
COPY . .
ARG VERSION=v1
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /service ./cmd/service

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /service /service
ENTRYPOINT ["/service"]
