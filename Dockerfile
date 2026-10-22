FROM golang:1.26 AS builder

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
ENV CGO_ENABLED=0 GOOS=linux

RUN go build -trimpath -ldflags "-s -w" -o /out/ofan ./cmd/server/

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /out/ofan /ofan
ENTRYPOINT ["/ofan"]
