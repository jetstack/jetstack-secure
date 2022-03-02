module github.com/jetstack/preflight

go 1.16

require (
	github.com/Azure/aks-engine v0.56.0
	github.com/Azure/azure-sdk-for-go v46.4.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.8 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.0 // indirect
	github.com/Jeffail/gabs/v2 v2.6.1
	github.com/aws/aws-sdk-go v1.36.19
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/d4l3k/messagediff v1.2.1
	github.com/fatih/color v1.13.0
	github.com/go-playground/universal-translator v0.17.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/jetstack/version-checker v0.2.2-0.20201118163251-4bab9ef088ef
	github.com/juju/errors v0.0.0-20190930114154-d42613fe1ab9
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8 // indirect
	github.com/juju/testing v0.0.0-20191001232224-ce9dec17d28b // indirect
	github.com/kylelemons/godebug v1.1.0
	github.com/leodido/go-urn v1.2.0 // indirect
	github.com/maxatome/go-testdeep v1.11.0
	github.com/pkg/errors v0.9.1
	github.com/pmylund/go-cache v2.1.0+incompatible
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	google.golang.org/api v0.70.0
	gopkg.in/d4l3k/messagediff.v1 v1.2.1
	gopkg.in/go-playground/validator.v9 v9.31.0 // indirect
	gopkg.in/mgo.v2 v2.0.0-20180705113604-9856a29383ce // indirect
	gopkg.in/yaml.v2 v2.4.0
	istio.io/istio v0.0.0-20210108091755-3c1dea2cb2bb
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	sigs.k8s.io/yaml v1.3.0
)

replace k8s.io/client-go => k8s.io/client-go v0.20.1
