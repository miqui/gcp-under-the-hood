FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/lbdemo .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/lbdemo /lbdemo
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/lbdemo"]
