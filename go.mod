module github.com/jetstack/preflight

go 1.13

require (
	github.com/Azure/aks-engine v0.56.0
	github.com/Azure/azure-sdk-for-go v46.4.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.8 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/Jeffail/gabs/v2 v2.6.0
	github.com/aws/aws-sdk-go v1.34.10
	github.com/cenkalti/backoff v2.0.0+incompatible
	github.com/d4l3k/messagediff v1.2.1
	github.com/go-logr/logr v0.2.1 // indirect
	github.com/go-playground/universal-translator v0.17.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.1.2 // indirect
	github.com/googleapis/gnostic v0.5.1 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jetstack/version-checker v0.2.2-0.20201118163251-4bab9ef088ef
	github.com/juju/errors v0.0.0-20190930114154-d42613fe1ab9
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8 // indirect
	github.com/juju/testing v0.0.0-20191001232224-ce9dec17d28b // indirect
	github.com/kylelemons/godebug v1.1.0
	github.com/leodido/go-urn v1.2.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0 // indirect
	golang.org/x/net v0.0.0-20201002202402-0a1ea396d57c // indirect
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43
	golang.org/x/sync v0.0.0-20200930132711-30421366ff76 // indirect
	golang.org/x/sys v0.0.0-20201005172224-997123666555 // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e // indirect
	google.golang.org/api v0.30.0
	gopkg.in/d4l3k/messagediff.v1 v1.2.1
	gopkg.in/go-playground/validator.v9 v9.31.0 // indirect
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce // indirect
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/klog/v2 v2.3.0 // indirect
	k8s.io/utils v0.0.0-20201005171033-6301aaf42dc7
)

replace k8s.io/client-go => k8s.io/client-go v0.19.2
