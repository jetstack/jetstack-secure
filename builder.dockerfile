FROM golang:1.18 as builder

WORKDIR /go/github.com/jetstack/preflight

# Run a dependency resolve with just the go mod files present for
# better caching
COPY ./go.mod .
COPY ./go.sum .

RUN go mod download

## Bring in everything else
COPY . .

ARG oauth_client_id
ARG oauth_client_secret
ARG oauth_auth_server_domain

RUN make build-all-platforms \
    BUILD_IN=host \
    OAUTH_CLIENT_ID=${oauth_client_id} \
    OAUTH_CLIENT_SECRET=${oauth_client_secret} \
    OAUTH_AUTH_SERVER_DOMAIN=${oauth_auth_server_domain}


# c781b427440f8ea100841eefdd308e660d26d121 is the v1.0.0 tag at the time of writing. 
RUN go install github.com/google/go-licenses@c781b427440f8ea100841eefdd308e660d26d121

# We need this '|| true' because go-licenses could fail to find a license so
# may return a non-zero exit code and there's no way to supress it.
RUN /go/bin/go-licenses save ./ --save_path="./builds/licenses/" || true
