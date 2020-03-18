# STAGE 1
FROM golang:1.13.4 as builder

WORKDIR /go/github.com/jetstack/preflight

# Run a dependency resolve with just the go mod files present for
# better caching
COPY ./go.mod .
COPY ./go.sum .

RUN go mod download

## Bring in everything else and build an amd64 image
COPY . .
RUN make install

# STAGE 2
# Use a distroless nonroot base image for just our executable
FROM gcr.io/distroless/base:nonroot
COPY --from=builder /go/bin/preflight /bin/preflight
ADD ./preflight-packages /preflight-packages
# load in an example config file
ADD ./agent.yaml /etc/preflight/agent.yaml
ENTRYPOINT ["preflight"]
CMD ["agent", "-c", "/etc/preflight/agent.yaml"]
