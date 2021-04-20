FROM gcr.io/distroless/base:nonroot

# TARGETPLATFORM comes from the buildx context and it will be something like `linux/arm64/v8` or `linux/amd64`.
# Ref: https://docs.docker.com/buildx/working-with-buildx/
ARG TARGETPLATFORM

USER preflight

COPY ./builds/${TARGETPLATFORM}/preflight /bin/preflight
# load in an example config file
ADD ./agent.yaml /etc/preflight/agent.yaml
ENTRYPOINT ["preflight"]
CMD ["agent", "-c", "/etc/preflight/agent.yaml"]
