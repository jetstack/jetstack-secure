package client_test

import (
	"context"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"

	"github.com/jetstack/venafi-connection-lib/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// These are using envtest (slow) rather than a fake clientset (fast) because
// controller-runtime's fake clientset doesn't support server-side apply [1] and
// also because we want to create serviceaccount tokens, which isn't supported
// by the fake clientset either.
//
// The goal is to test the following behaviors:
//
//   - VenafiConnection's `accessToken` works as expected with a fake Venafi
//     Cloud server.
//   - VenafiConnection's `apiKey` and `tpp` can't be used by the user.
//   - NewVenConnClient's `trustedCAs` works as expected.
//
// [1] https://github.com/kubernetes-sigs/controller-runtime/issues/2341
func TestVenConnClient_PostDataReadingsWithOptions(t *testing.T) {
	t.Parallel()

	t.Run("valid accessToken", run(testcase{
		given: undent(`
			apiVersion: jetstack.io/v1alpha1
			kind: VenafiConnection
			metadata:
			  name: venafi-components
			  namespace: venafi
			spec:
			  vcp:
			    url: FAKE_VENAFI_CLOUD_URL
			    accessToken:
			      - secret:
			          name: accesstoken
			          fields: [accesstoken]`),
		expectReadyCondMsg: "ea744d098c2c1c6044e4c4e9d3bf7c2a68ef30553db00f1714886cedf73230f1",
	}))
	t.Run("error when the apiKey field is used", run(testcase{
		// Why isn't it possible to use the 'apiKey' field? Although the
		// Kubernetes Discovery endpoint works with an API key, we have decided
		// to not support it because it isn't recommended.
		given: undent(`
			apiVersion: jetstack.io/v1alpha1
			kind: VenafiConnection
			metadata:
			  name: venafi-components
			  namespace: venafi
			spec:
			  vcp:
			    url: FAKE_VENAFI_CLOUD_URL
			    apiKey:
			      - secret:
			          name: apikey
			          fields: [apikey]`),
		expectReadyCondMsg: "b099d634ccec56556da28028743475dab67f79d079b668bedc3ef544f7eed2f3",
		expectErr:          "VenafiConnection venafi/venafi-components: the agent cannot be used with an API key",
	}))
	t.Run("error when the tpp field is used", run(testcase{
		// IMPORTANT: The user may think they can use 'tpp', spend time
		// debugging and making the venafi connection work, and then find out
		// that it doesn't work. The reason is because as of now, we don't first
		// check if the user has used the 'tpp' field before running Get.
		given: undent(`
			apiVersion: jetstack.io/v1alpha1
			kind: VenafiConnection
			metadata:
			  name: venafi-components
			  namespace: venafi
			spec:
			  tpp:
			    url: FAKE_TPP_URL
			    accessToken:
			      - secret:
			          name: accesstoken
			          fields: [accesstoken]`),
		expectErr:          ``,
		expectReadyCondMsg: `ea744d098c2c1c6044e4c4e9d3bf7c2a68ef30553db00f1714886cedf73230f1`,
	}))
}

type testcase struct {
	given              string
	expectErr          string
	expectReadyCondMsg string
}

func run(test testcase) func(t *testing.T) {
	return func(t *testing.T) {
		fakeVenafiCloud, certCloud := fakeVenafiCloud(t)
		fakeTPP, certTPP := fakeTPP(t)
		_, restconf, kclient := startEnvtest(t)

		certPool := x509.NewCertPool()
		certPool.AddCert(certCloud)
		certPool.AddCert(certTPP)

		cl, err := client.NewVenConnClient(
			restconf,
			&api.AgentMetadata{ClusterID: "no"},
			"venafi",            // Namespace in which the Agent is running.
			"venafi-components", // Name of the VenafiConnection.
			"venafi",            // Namespace of the VenafiConnection.
			certPool,
		)
		require.NoError(t, err)

		// This `cancel` is important because the below func `Start(ctx)` needs
		// to be stopped before the apiserver is stopped. Otherwise, the test
		// fail with the message "timeout waiting for process kube-apiserver to
		// stop". See:
		// https://github.com/jetstack/venafi-connection-lib/pull/158#issuecomment-1949002322
		// https://github.com/kubernetes-sigs/controller-runtime/issues/1571#issuecomment-945535598
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			err = cl.Start(ctx)
			require.NoError(t, err)
		}()
		t.Cleanup(cancel)

		// Apply the same RBAC as what you would get from the Venafi
		// Connection Helm chart, for example after running this:
		//  helm template venafi-connection oci://registry.venafi.cloud/charts/venafi-connection --version v0.1.0 -n venafi --show-only templates/venafi-connection-rbac.yaml
		require.NoError(t, kclient.Create(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "venafi"},
		}))
		require.NoError(t, kclient.Create(context.Background(), &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "venafi-connection", Namespace: "venafi"},
		}))
		require.NoError(t, kclient.Create(context.Background(), &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "venafi-connection-role"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"namespaces"}, Verbs: []string{"get", "list", "watch"}},
				{APIGroups: []string{"jetstack.io"}, Resources: []string{"venaficonnections"}, Verbs: []string{"get", "list", "watch"}},
				{APIGroups: []string{"jetstack.io"}, Resources: []string{"venaficonnections/status"}, Verbs: []string{"get", "patch"}},
			},
		}))
		require.NoError(t, kclient.Create(context.Background(), &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "venafi-connection-rolebinding"},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "venafi-connection-role"},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "venafi-connection", Namespace: "venafi"}},
		}))
		require.NoError(t, kclient.Create(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "accesstoken", Namespace: "venafi"},
			StringData: map[string]string{"accesstoken": "VALID_ACCESS_TOKEN"},
		}))
		require.NoError(t, kclient.Create(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "apikey", Namespace: "venafi"},
			StringData: map[string]string{"apikey": "VALID_API_KEY"},
		}))
		require.NoError(t, kclient.Create(context.Background(), &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "venafi-connection-secret-reader", Namespace: "venafi"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}, ResourceNames: []string{"accesstoken", "apikey"}},
			},
		}))
		require.NoError(t, kclient.Create(context.Background(), &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "venafi-connection-secret-reader", Namespace: "venafi"},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "venafi-connection-secret-reader"},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "venafi-connection", Namespace: "venafi"}},
		}))

		test.given = strings.ReplaceAll(test.given, "FAKE_VENAFI_CLOUD_URL", fakeVenafiCloud.URL)
		test.given = strings.ReplaceAll(test.given, "FAKE_TPP_URL", fakeTPP.URL)
		for _, obj := range parse(test.given) {
			require.NoError(t, kclient.Create(context.Background(), obj))
		}

		err = cl.PostDataReadingsWithOptions([]*api.DataReading{}, client.Options{ClusterName: "test cluster name"})
		if test.expectErr != "" {
			assert.EqualError(t, err, test.expectErr)
		}

		got := v1alpha1.VenafiConnection{}
		kclient.Get(context.Background(), types.NamespacedName{Name: "venafi-components", Namespace: "venafi"}, &got)
		require.Len(t, got.Status.Conditions, 1)
		assert.Equal(t, test.expectReadyCondMsg, got.Status.Conditions[0].Message)
	}
}

func fakeVenafiCloud(t *testing.T) (*httptest.Server, *x509.Certificate) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("fake api.venafi.cloud received request: %s %s", r.Method, r.URL.Path)
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

	return server, cert
}

func fakeTPP(t testing.TB) (*httptest.Server, *x509.Certificate) {
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

// To see the API server logs, set:
//
//	export KUBEBUILDER_ATTACH_CONTROL_PLANE_OUTPUT=true
func startEnvtest(t testing.TB) (_ *envtest.Environment, _ *rest.Config, kclient ctrlruntime.WithWatch) {
	envtest := &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths:     []string{"/tmp/venafi-connection.yaml"},
	}
	restconf, err := envtest.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		t.Log("Waiting for envtest to exit")
		err = envtest.Stop()
		require.NoError(t, err)
	})

	sch := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(sch)
	_ = corev1.AddToScheme(sch)
	_ = rbacv1.AddToScheme(sch)

	kclient, err = ctrlruntime.NewWithWatch(restconf, ctrlruntime.Options{Scheme: sch})
	require.NoError(t, err)

	return envtest, restconf, kclient
}

// Undent removes leading indentation/white-space from given string and returns
// it as a string. Useful for inlining YAML manifests in Go code. Inline YAML
// manifests in the Go test files makes it easier to read the test case as
// opposed to reading verbose-y Go structs.
//
// This was copied from https://github.com/jimeh/undent/blob/main/undent.go, all
// credit goes to the author, Jim Myhrberg.
func undent(s string) string {
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

		if offset+min < end {
			out = append(out, s[offset+min:end]...)
		} else if offset < end {
			out = append(out, s[offset:end]...)
		}
	}

	return string(out)
}

// Parses the YAML manifest. Useful for inlining YAML manifests in Go test
// files, to be used in conjunction with `undent`.
func parse(yamlmanifest string) []ctrlruntime.Object {
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
