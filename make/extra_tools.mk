ADDITIONAL_TOOLS :=
ADDITIONAL_GO_DEPENDENCIES :=

# https://pkg.go.dev/github.com/helm-unittest/helm-unittest?tab=versions
ADDITIONAL_TOOLS += helm-unittest=v0.8.2
ADDITIONAL_GO_DEPENDENCIES += helm-unittest=github.com/helm-unittest/helm-unittest/cmd/helm-unittest

ADDITIONAL_TOOLS += venctl=1.27.0
ADDITIONAL_TOOLS += step=0.28.2

