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

ARG oauth_client_id
ARG oauth_client_secret
ARG oauth_auth_server_domain

# RUN CGO_ENABLED=0 go install .
RUN make install \
  OAUTH_CLIENT_ID=${oauth_client_id} \
  OAUTH_CLIENT_SECRET=${oauth_client_secret} \
  OAUTH_AUTH_SERVER_DOMAIN=${oauth_auth_server_domain}

# STAGE 2
# Use a distroless nonroot base image for just our executable
FROM gcr.io/distroless/base:nonroot
COPY --from=builder /go/bin/preflight /bin/preflight
# load in an example config file
ADD ./agent.yaml /etc/preflight/agent.yaml
ENTRYPOINT ["preflight"]
CMD ["agent", "-c", "/etc/preflight/agent.yaml"]
