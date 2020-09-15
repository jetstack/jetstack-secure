module github.com/jetstack/preflight

go 1.13

replace k8s.io/client-go => k8s.io/client-go v0.19.1

require (
	github.com/Azure/aks-engine v0.55.3
	github.com/aws/aws-sdk-go v1.25.30
	github.com/cenkalti/backoff v2.0.0+incompatible
	github.com/d4l3k/messagediff v1.2.1 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/juju/errors v0.0.0-20190930114154-d42613fe1ab9
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8 // indirect
	github.com/juju/testing v0.0.0-20191001232224-ce9dec17d28b // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/kylelemons/godebug v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	golang.org/x/oauth2 v0.0.0-20191202225959-858c2ad4c8b6
	google.golang.org/api v0.15.0
	gopkg.in/d4l3k/messagediff.v1 v1.2.1
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce // indirect
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/apimachinery v0.19.1
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
)
