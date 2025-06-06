package client_test

import (
	"context"
	"crypto/x509"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/jetstack/venafi-connection-lib/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/testutil"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log := ktesting.NewLogger(t, ktesting.NewConfig(ktesting.Verbosity(10)))
	ctx = klog.NewContext(ctx, log)
	_, restconf, kclient := testutil.WithEnvtest(t)
	for _, obj := range testutil.Parse(testutil.VenConnRBAC) {
		require.NoError(t, kclient.Create(ctx, obj))
	}
	t.Parallel()

	t.Run("valid accessToken", run_TestVenConnClient_PostDataReadingsWithOptions(ctx, restconf, kclient, testcase{
		given: testutil.Undent(`
			apiVersion: jetstack.io/v1alpha1
			kind: VenafiConnection
			metadata:
			  name: venafi-components
			  namespace: TEST_NAMESPACE
			spec:
			  vcp:
			    url: FAKE_VENAFI_CLOUD_URL
			    accessToken:
			      - secret:
			          name: accesstoken
			          fields: [accesstoken]
			  allowReferencesFrom:
			    matchExpressions:
			      - {key: kubernetes.io/metadata.name, operator: In, values: [venafi]}
			---
			apiVersion: v1
			kind: Secret
			metadata:
			  name: accesstoken
			  namespace: TEST_NAMESPACE
			stringData:
			  accesstoken: VALID_ACCESS_TOKEN
			---
			apiVersion: rbac.authorization.k8s.io/v1
			kind: Role
			metadata:
			  name: venafi-connection-accesstoken-reader
			  namespace: TEST_NAMESPACE
			rules:
			- apiGroups: [""]
			  resources: ["secrets"]
			  verbs: ["get"]
			  resourceNames: ["accesstoken"]
			---
			apiVersion: rbac.authorization.k8s.io/v1
			kind: RoleBinding
			metadata:
			  name: venafi-connection-accesstoken-reader
			  namespace: TEST_NAMESPACE
			roleRef:
			  apiGroup: rbac.authorization.k8s.io
			  kind: Role
			  name: venafi-connection-accesstoken-reader
			subjects:
			- kind: ServiceAccount
			  name: venafi-connection
			  namespace: venafi
		`),
		expectReadyCondMsg: "Generated a new token",
	}))
	t.Run("error when the apiKey field is used", run_TestVenConnClient_PostDataReadingsWithOptions(ctx, restconf, kclient, testcase{
		// Why isn't it possible to use the 'apiKey' field? Although the
		// Kubernetes Discovery endpoint works with an API key, we have decided
		// to not support it because it isn't recommended.
		given: testutil.Undent(`
			apiVersion: jetstack.io/v1alpha1
			kind: VenafiConnection
			metadata:
			  name: venafi-components
			  namespace: TEST_NAMESPACE
			spec:
			  vcp:
			    url: FAKE_VENAFI_CLOUD_URL
			    apiKey:
			      - secret:
			          name: apikey
			          fields: [apikey]
			  allowReferencesFrom:
			    matchExpressions:
			      - {key: kubernetes.io/metadata.name, operator: In, values: [venafi]}
			---
			apiVersion: v1
			kind: Secret
			metadata:
			  name: apikey
			  namespace: TEST_NAMESPACE
			stringData:
			  apikey: VALID_API_KEY
			---
			apiVersion: rbac.authorization.k8s.io/v1
			kind: Role
			metadata:
			  name: venafi-connection-apikey-reader
			  namespace: TEST_NAMESPACE
			rules:
			- apiGroups: [""]
			  resources: ["secrets"]
			  verbs: ["get"]
			  resourceNames: ["apikey"]
			---
			apiVersion: rbac.authorization.k8s.io/v1
			kind: RoleBinding
			metadata:
			  name: venafi-connection-apikey-reader
			  namespace: TEST_NAMESPACE
			roleRef:
			  apiGroup: rbac.authorization.k8s.io
			  kind: Role
			  name: venafi-connection-apikey-reader
			subjects:
			- kind: ServiceAccount
			  name: venafi-connection
			  namespace: venafi
		`),
		// PostDataReadingsWithOptions failed, but Get succeeded; that's why the
		// condition says the VenafiConnection is ready.
		expectReadyCondMsg: "Generated a new token",
		expectErr:          "VenafiConnection error-when-the-apikey-field-is-used/venafi-components: the agent cannot be used with an API key",
	}))
	t.Run("error when the tpp field is used", run_TestVenConnClient_PostDataReadingsWithOptions(ctx, restconf, kclient, testcase{
		// IMPORTANT: The user may think they can use 'tpp', spend time
		// debugging and making the venafi connection work, and then find out
		// that it doesn't work. The reason is because as of now, we don't first
		// check if the user has used the 'tpp' field before running Get.
		given: testutil.Undent(`
			apiVersion: jetstack.io/v1alpha1
			kind: VenafiConnection
			metadata:
			  name: venafi-components
			  namespace: TEST_NAMESPACE
			spec:
			  tpp:
			    url: FAKE_TPP_URL
			    accessToken:
			      - secret:
			          name: accesstoken
			          fields: [accesstoken]
			  allowReferencesFrom:
			    matchExpressions:
			      - {key: kubernetes.io/metadata.name, operator: In, values: [venafi]}
			---
			apiVersion: v1
			kind: Secret
			metadata:
			  name: accesstoken
			  namespace: TEST_NAMESPACE
			stringData:
			  accesstoken: VALID_ACCESS_TOKEN
			---
			apiVersion: rbac.authorization.k8s.io/v1
			kind: Role
			metadata:
			  name: venafi-connection-accesstoken-reader
			  namespace: TEST_NAMESPACE
			rules:
			- apiGroups: [""]
			  resources: ["secrets"]
			  verbs: ["get"]
			  resourceNames: ["accesstoken"]
			---
			apiVersion: rbac.authorization.k8s.io/v1
			kind: RoleBinding
			metadata:
			  name: venafi-connection-accesstoken-reader
			  namespace: TEST_NAMESPACE
			roleRef:
			  apiGroup: rbac.authorization.k8s.io
			  kind: Role
			  name: venafi-connection-accesstoken-reader
			subjects:
			- kind: ServiceAccount
			  name: venafi-connection
			  namespace: venafi
		`),
		expectReadyCondMsg: "Generated a new token",
		expectErr:          "VenafiConnection error-when-the-tpp-field-is-used/venafi-components: the agent cannot be used with TPP",
	}))
}

type testcase struct {
	given              string
	expectErr          string
	expectReadyCondMsg string
}

// All tests share the same envtest (i.e., the same apiserver and etcd process),
// so each test needs to be contained in its own Kubernetes namespace.
func run_TestVenConnClient_PostDataReadingsWithOptions(ctx context.Context, restcfg *rest.Config, kclient ctrlruntime.WithWatch, test testcase) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()
		log := ktesting.NewLogger(t, ktesting.NewConfig(ktesting.Verbosity(10)))
		ctx := klog.NewContext(ctx, log)
		fakeVenafiCloud, certCloud, fakeVenafiAssert := testutil.FakeVenafiCloud(t)
		fakeTPP, certTPP := testutil.FakeTPP(t)
		fakeVenafiAssert(func(t testing.TB, r *http.Request) {
			if r.URL.Path == "/v1/useraccounts" {
				return // We only care about /v1/tlspk/upload/clusterdata.
			}
			// Let's make sure we didn't forget to add the arbitrary "/no"
			// (uploader_id) path segment to /v1/tlspk/upload/clusterdata.
			assert.Equal(t, "/v1/tlspk/upload/clusterdata/no", r.URL.Path)
		})

		certPool := x509.NewCertPool()
		certPool.AddCert(certCloud)
		certPool.AddCert(certTPP)

		cl, err := client.NewVenConnClient(
			restcfg,
			&api.AgentMetadata{ClusterID: "no"},
			"venafi",               // Namespace in which the Agent is running.
			"venafi-components",    // Name of the VenafiConnection.
			testNameToNamespace(t), // Namespace of the VenafiConnection.
			certPool,
		)
		require.NoError(t, err)

		testutil.VenConnStartWatching(ctx, t, cl)

		test.given = strings.ReplaceAll(test.given, "FAKE_VENAFI_CLOUD_URL", fakeVenafiCloud.URL)
		test.given = strings.ReplaceAll(test.given, "FAKE_TPP_URL", fakeTPP.URL)
		test.given = strings.ReplaceAll(test.given, "TEST_NAMESPACE", testNameToNamespace(t))

		var givenObjs []ctrlruntime.Object
		givenObjs = append(givenObjs, testutil.Parse(testutil.Undent(`
			apiVersion: v1
			kind: Namespace
			metadata:
			  name: `+testNameToNamespace(t)))...)
		givenObjs = append(givenObjs, testutil.Parse(test.given)...)
		for _, obj := range givenObjs {
			require.NoError(t, kclient.Create(ctx, obj))
		}
		err = cl.PostDataReadingsWithOptions(ctx, []*api.DataReading{}, client.Options{ClusterName: "test cluster name"})
		if test.expectErr != "" {
			assert.EqualError(t, err, test.expectErr)
		} else {
			require.NoError(t, err)
		}

		got := v1alpha1.VenafiConnection{}
		err = kclient.Get(ctx, types.NamespacedName{Name: "venafi-components", Namespace: testNameToNamespace(t)}, &got)
		require.NoError(t, err)
		require.Len(t, got.Status.Conditions, 1)
		assert.Equal(t, test.expectReadyCondMsg, got.Status.Conditions[0].Message)
	}
}

// Because we want valid namespaces for each of the tests, this func converts a
// test name into a valid Kubernetes namespace (i.e., a DNS label as per RFC
// 1123, including trimming to 63 chars).
//
// For example, the test name:
//
//	Test/sub test has special chars ':"-;@# and is also super super super super long!
//
// will be converted to:
//
//	sub-test-has-special-chars-and-is-also-super-super-super-super-
//
// Only the last part of the test name is used.
//
// nolint:dupword
func testNameToNamespace(t testing.TB) string {
	regex := regexp.MustCompile("[^a-zA-Z0-9-]")

	// Only keep the part after the last slash.
	parts := strings.Split(t.Name(), "/")
	if len(parts) == 0 {
		return ""
	}

	s := parts[len(parts)-1]
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "-")
	s = regex.ReplaceAllString(s, "")
	s = strings.TrimLeft(s, "-")
	s = strings.TrimRight(s, "-")
	return s
}
