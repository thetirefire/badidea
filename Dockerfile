FROM golang:1.15.2 as builder
WORKDIR /workspace

# Copy the sources
COPY ./ ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o badidea ./

FROM gcr.io/distroless/base:debug
WORKDIR /
COPY --from=builder /workspace/badidea .
ENTRYPOINT ["/badidea"]