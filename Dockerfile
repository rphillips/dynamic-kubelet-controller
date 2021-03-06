# Build the manager binary
FROM docker.io/openshift/origin-release:golang-1.10 as builder

# Copy in the go src
WORKDIR /go/src/github.com/rphillips/dynamic-kubelet-controller
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/rphillips/dynamic-kubelet-controller/cmd/manager

# Copy the controller-manager into a thin image
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /go/src/github.com/rphillips/dynamic-kubelet-controller/manager .
ENTRYPOINT ["./manager"]
