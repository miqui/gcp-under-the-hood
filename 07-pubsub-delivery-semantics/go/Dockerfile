# Optional: run the demo binary in a container (e.g. Cloud Run Jobs / GKE).
FROM golang:1.22 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /psds .

FROM gcr.io/distroless/static-debian12
COPY --from=build /psds /psds
ENTRYPOINT ["/psds"]
