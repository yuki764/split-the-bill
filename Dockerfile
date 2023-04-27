FROM golang:1.20-bullseye as build

WORKDIR /go/src/
COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 go build .

FROM gcr.io/distroless/static-debian11
COPY --from=build /go/src/split-the-bill /
COPY templates/ /templates/
EXPOSE 8080
CMD ["/split-the-bill"]
