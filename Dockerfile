FROM gcr.io/distroless/static

# TARGETPLATFORM comes from the buildx context and it will be something like `linux/arm64/v8` or `linux/amd64`.
# Ref: https://docs.docker.com/buildx/working-with-buildx/
ARG TARGETPLATFORM

USER 1000:1000

COPY ./builds/${TARGETPLATFORM}/preflight /bin/preflight
# load in an example config file
ADD ./agent.yaml /etc/preflight/agent.yaml
ENTRYPOINT ["preflight"]
CMD ["agent", "-c", "/etc/preflight/agent.yaml"]
