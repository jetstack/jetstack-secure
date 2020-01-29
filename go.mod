module github.com/jetstack/preflight

go 1.13

require (
	cloud.google.com/go/storage v1.4.0
	github.com/Azure/aks-engine v0.43.1
	github.com/Azure/azure-pipeline-go v0.2.2 // indirect
	github.com/Azure/azure-storage-blob-go v0.8.0
	github.com/aws/aws-sdk-go v1.25.30
	github.com/blang/semver v3.5.1+incompatible
	github.com/gomarkdown/markdown v0.0.0-20191104174740-4d42851d4d5a
	github.com/google/go-cmp v0.3.0
	github.com/gookit/color v1.2.0
	github.com/ianlancetaylor/demangle v0.0.0-20181102032728-5e5cf60278f6 // indirect
	github.com/juju/errors v0.0.0-20190930114154-d42613fe1ab9
	github.com/juju/loggo v0.0.0-20190526231331-6e530bcce5d8 // indirect
	github.com/juju/testing v0.0.0-20191001232224-ce9dec17d28b // indirect
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-ieproxy v0.0.0-20191113090002-7c0f6868bffe // indirect
	github.com/open-policy-agent/opa v0.16.0
	github.com/pkg/errors v0.8.1
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.5.0
	github.com/yudai/gojsondiff v1.0.0
	golang.org/x/net v0.0.0-20200114155413-6afb5195e5aa // indirect
	golang.org/x/oauth2 v0.0.0-20191202225959-858c2ad4c8b6
	golang.org/x/sys v0.0.0-20200120151820-655fe14d7479 // indirect
	google.golang.org/api v0.15.0
	gopkg.in/yaml.v2 v2.2.7
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v10.0.0+incompatible
)

// This is needed because otherwise k8s.io/client-go is forced
// to v10.0.0+incompatible because of aks-engine, and that
// causes another set of problems with inconsistent dependencies.
replace k8s.io/client-go => k8s.io/client-go v0.17.0
