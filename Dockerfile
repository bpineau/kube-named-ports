FROM golang:1.10.0 as builder
WORKDIR /go/src/github.com/mirakl/kube-named-ports
COPY . .
RUN go get -u github.com/Masterminds/glide
RUN make deps
RUN make build

FROM alpine:3.7
RUN apk --no-cache add ca-certificates
COPY --from=builder /go/src/github.com/mirakl/kube-named-ports/kube-named-ports /usr/bin/
ENTRYPOINT ["/usr/bin/kube-named-ports"]
