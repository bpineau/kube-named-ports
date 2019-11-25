FROM golang:1.13.4 as builder
WORKDIR /go/src/github.com/bpineau/kube-named-ports
COPY . .
RUN make build

FROM alpine:3.10.3
RUN apk --no-cache add ca-certificates
COPY --from=builder /go/src/github.com/bpineau/kube-named-ports/kube-named-ports /usr/bin/
ENTRYPOINT ["/usr/bin/kube-named-ports"]
