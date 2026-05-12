FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /external-dns-tencentcloud-webhook .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /external-dns-tencentcloud-webhook /external-dns-tencentcloud-webhook
ENTRYPOINT ["/external-dns-tencentcloud-webhook"]
