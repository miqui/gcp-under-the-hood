FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/dbapp ./cmd/dbapp \
 && CGO_ENABLED=0 go build -o /out/signtool ./cmd/signtool

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /app/
ENTRYPOINT ["/app/dbapp"]
