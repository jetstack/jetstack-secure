FROM golang:1.23.1 as builder

WORKDIR /go/github.com/jetstack/preflight

# Run a dependency resolve with just the go mod files present for
# better caching
COPY go.mod go.sum .

COPY <<EOF /root/.gitconfig
[url "git@github.com:jetstack/venafi-connection-lib"] \
insteadOf = https://github.com/jetstack/venafi-connection-lib
EOF
COPY <<EOF /root/.ssh/known_hosts
github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl
github.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=
github.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=
EOF
ENV GOPRIVATE=github.com/jetstack/venafi-connection-lib

RUN --mount=type=ssh go mod download

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

RUN go install github.com/google/go-licenses@v1.6.0

# We need this '|| true' because go-licenses could fail to find a license so
# may return a non-zero exit code and there's no way to supress it.
RUN /go/bin/go-licenses save ./ --save_path="./builds/licenses/" || true
