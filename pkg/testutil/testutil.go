package testutil

import (
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jetstack/venafi-connection-lib/api/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// To see the API server logs, set:
//
//	export KUBEBUILDER_ATTACH_CONTROL_PLANE_OUTPUT=true
func WithEnvtest(t testing.TB) (_ *envtest.Environment, _ *rest.Config, kclient ctrlruntime.WithWatch) {
	t.Helper()

	// If KUBEBUILDER_ASSETS isn't set, show a warning to the user.
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Fatalf("KUBEBUILDER_ASSETS isn't set. You can run this test using `make test`.\n" +
			"But if you prefer not to use `make`, run these two commands first:\n" +
			"    make _bin/tools/{kube-apiserver,etcd}\n" +
			"    export KUBEBUILDER_ASSETS=$PWD/_bin/tools")
	}
	envtest := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     []string{"../../deploy/charts/venafi-kubernetes-agent/crd_bases/jetstack.io_venaficonnections.yaml"},
	}

	restconf, err := envtest.Start()
	t.Cleanup(func() {
		t.Log("Waiting for envtest to exit")
		e := envtest.Stop()
		require.NoError(t, e)
	})
	require.NoError(t, err)

	sch := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(sch)
	_ = corev1.AddToScheme(sch)
	_ = rbacv1.AddToScheme(sch)

	kclient, err = ctrlruntime.NewWithWatch(restconf, ctrlruntime.Options{Scheme: sch})
	require.NoError(t, err)

	return envtest, restconf, kclient
}

// Copied from https://github.com/kubernetes/client-go/issues/711#issuecomment-1666075787.
func WithKubeconfig(t testing.TB, restCfg *rest.Config) string {
	t.Helper()

	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters["default-cluster"] = &clientcmdapi.Cluster{
		Server:                   restCfg.Host,
		CertificateAuthorityData: restCfg.CAData,
	}
	contexts := make(map[string]*clientcmdapi.Context)
	contexts["default-context"] = &clientcmdapi.Context{
		Cluster:  "default-cluster",
		AuthInfo: "default-user",
	}
	authinfos := make(map[string]*clientcmdapi.AuthInfo)
	authinfos["default-user"] = &clientcmdapi.AuthInfo{
		ClientCertificateData: restCfg.CertData,
		ClientKeyData:         restCfg.KeyData,
	}
	clientConfig := clientcmdapi.Config{
		Kind:           "Config",
		APIVersion:     "v1",
		Clusters:       clusters,
		Contexts:       contexts,
		CurrentContext: "default-context",
		AuthInfos:      authinfos,
	}

	d := t.TempDir()
	kubeconfig, _ := os.CreateTemp(d, "kubeconfig")
	defer kubeconfig.Close()

	err := clientcmd.WriteToFile(clientConfig, kubeconfig.Name())
	require.NoError(t, err)

	return kubeconfig.Name()
}

// Undent removes leading indentation/white-space from given string and returns
// it as a string. Useful for inlining YAML manifests in Go code. Inline YAML
// manifests in the Go test files makes it easier to read the test case as
// opposed to reading verbose-y Go structs.
//
// This was copied from https://github.com/jimeh/Undent/blob/main/Undent.go, all
// credit goes to the author, Jim Myhrberg.
func Undent(s string) string {
	const (
		tab = 9
		lf  = 10
		spc = 32
	)

	if len(s) == 0 {
		return ""
	}

	// find smallest indent relative to each line-feed
	min := 99999999999
	count := 0

	lfs := make([]int, 0, strings.Count(s, "\n"))
	if s[0] != lf {
		lfs = append(lfs, -1)
	}

	indent := 0
	for i := 0; i < len(s); i++ {
		if s[i] == lf {
			lfs = append(lfs, i)
			indent = 0
		} else if indent < min {
			switch s[i] {
			case spc, tab:
				indent++
			default:
				if indent > 0 {
					count++
				}
				if indent < min {
					min = indent
				}
			}
		}
	}

	// extract each line without indentation
	out := make([]byte, 0, len(s)-(min*count))

	for i := 0; i < len(lfs); i++ {
		offset := lfs[i] + 1
		end := len(s)
		if i+1 < len(lfs) {
			end = lfs[i+1] + 1
		}

		if offset+min <= end {
			out = append(out, s[offset+min:end]...)
		} else if offset < end {
			out = append(out, s[offset:end]...)
		}
	}

	return string(out)
}

// Parses the YAML manifest. Useful for inlining YAML manifests in Go test
// files, to be used in conjunction with `undent`.
func Parse(yamlmanifest string) []ctrlruntime.Object {
	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(yamlmanifest), 4096)
	var objs []ctrlruntime.Object
	for {
		obj := &unstructured.Unstructured{}
		err := dec.Decode(obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}

		objs = append(objs, obj)
	}
	return objs
}

type AssertRequest func(t testing.TB, r *http.Request)

func FakeVenafiCloud(t *testing.T) (_ *httptest.Server, _ *x509.Certificate, setAssert func(AssertRequest)) {
	t.Helper()

	var assertFn AssertRequest
	assertFnMu := sync.Mutex{}
	setAssert = func(setAssert AssertRequest) {
		assertFnMu.Lock()
		defer assertFnMu.Unlock()
		assertFn = setAssert
	}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("fake api.venafi.cloud received request: %s %s", r.Method, r.URL.Path)

		assertFnMu.Lock()
		defer assertFnMu.Unlock()
		assertFn(t, r)

		accessToken := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		apiKey := r.Header.Get("tppl-api-key")
		if accessToken != "VALID_ACCESS_TOKEN" && apiKey != "VALID_API_KEY" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/v1/tlspk/upload/clusterdata/no" {
			if r.URL.Query().Get("name") != "test cluster name" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"status":"ok","organization":"756db001-280e-11ee-84fb-991f3177e2d0"}`))
		} else if r.URL.Path == "/v1/useraccounts" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"user": {"username": "user","id": "76a126f0-280e-11ee-84fb-991f3177e2d0"}}`))

		} else if r.URL.Path == "/v1/oauth2/v2.0/756db001-280e-11ee-84fb-991f3177e2d0/token" {
			_, _ = w.Write([]byte(`{"access_token":"VALID_ACCESS_TOKEN","expires_in":900,"token_type":"bearer"}`))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"unexpected path in the test server","path":"` + r.URL.Path + `"}`))
		}
	}))
	t.Cleanup(server.Close)

	cert, err := x509.ParseCertificate(server.TLS.Certificates[0].Certificate[0])
	require.NoError(t, err)

	return server, cert, setAssert
}

func FakeTPP(t testing.TB) (*httptest.Server, *x509.Certificate) {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("fake tpp.example.com received request: %s %s", r.Method, r.URL.Path)

		accessToken := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		if r.URL.Path == "/vedsdk/Identity/Self" {
			if accessToken != "VALID_ACCESS_TOKEN" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"Identities":[{"Name":"TEST"}]}`))
		} else if r.URL.Path == "/vedsdk/certificates/checkpolicy" {
			_, _ = w.Write([]byte(`{"Policy":{"Subject":{"Organization":{"Value": "test-org"}}}}`))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"unexpected path in the test server","path":"` + r.URL.Path + `"}`))
		}
	}))
	t.Cleanup(server.Close)

	cert, err := x509.ParseCertificate(server.TLS.Certificates[0].Certificate[0])
	require.NoError(t, err)

	return server, cert
}

// Generated using:
//
//	helm template ./deploy/charts/venafi-kubernetes-agent -n venafi --set venafiConnection.include=true --show-only templates/venafi-connection-VenConnRBAC.yaml | grep -ivE '(helm|\/version)'
const VenConnRBAC = `
apiVersion: v1
kind: Namespace
metadata:
  name: venafi
---
# Source: venafi-kubernetes-agent/templates/venafi-connection-rbac.yaml
# The 'venafi-connection' service account is used by multiple
# controllers. When configuring which resources a VenafiConnection
# can access, the RBAC rules you create manually must point to this SA.
apiVersion: v1
kind: ServiceAccount
metadata:
  name: venafi-connection
  namespace: "venafi"
  labels:
    app.kubernetes.io/name: "venafi-connection"
    app.kubernetes.io/instance: release-name
---
# Source: venafi-kubernetes-agent/templates/venafi-connection-rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: venafi-connection-role
  labels:
    app.kubernetes.io/name: "venafi-connection"
    app.kubernetes.io/instance: release-name
rules:
- apiGroups: [ "" ]
  resources: [ "namespaces" ]
  verbs: [ "get", "list", "watch" ]

- apiGroups: [ "jetstack.io" ]
  resources: [ "venaficonnections" ]
  verbs: [ "get", "list", "watch" ]

- apiGroups: [ "jetstack.io" ]
  resources: [ "venaficonnections/status" ]
  verbs: [ "get", "patch" ]
---
# Source: venafi-kubernetes-agent/templates/venafi-connection-rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: venafi-connection-rolebinding
  labels:
    app.kubernetes.io/name: "venafi-connection"
    app.kubernetes.io/instance: release-name
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: venafi-connection-role
subjects:
- kind: ServiceAccount
  name: venafi-connection
  namespace: "venafi"
`
