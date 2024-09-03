package client_test

import (
	"context"
	"crypto/x509"
	"strings"
	"testing"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/client"
	"github.com/jetstack/preflight/pkg/testutil"

	"github.com/jetstack/venafi-connection-lib/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/client"
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
		given: testutil.Undent(`
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
		given: testutil.Undent(`
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
		given: testutil.Undent(`
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

// Generated using:
//
//	helm template ./deploy/charts/venafi-kubernetes-agent -n venafi --set venafiConnection.include=true --show-only templates/venafi-connection-rbac.yaml | grep -ivE '(helm|\/version)'
const rbac = `
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

type testcase struct {
	given              string
	expectErr          string
	expectReadyCondMsg string
}

func run(test testcase) func(t *testing.T) {
	return func(t *testing.T) {
		fakeVenafiCloud, certCloud, _ := testutil.FakeVenafiCloud(t)
		fakeTPP, certTPP := testutil.FakeTPP(t)
		_, restconf, kclient := testutil.WithEnvtest(t)

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

		test.given = strings.ReplaceAll(test.given, "FAKE_VENAFI_CLOUD_URL", fakeVenafiCloud.URL)
		test.given = strings.ReplaceAll(test.given, "FAKE_TPP_URL", fakeTPP.URL)

		var given []ctrlruntime.Object
		given = append(given, testutil.Parse(rbac)...)
		given = append(given, testutil.Parse(testutil.Undent(`
			apiVersion: v1
			kind: Secret
			metadata:
			  name: accesstoken
			  namespace: venafi
			stringData:
			  accesstoken: VALID_ACCESS_TOKEN
			---
			apiVersion: v1
			kind: Secret
			metadata:
			  name: apikey
			  namespace: venafi
			stringData:
			  apikey: VALID_API_KEY
			---
			apiVersion: rbac.authorization.k8s.io/v1
			kind: Role
			metadata:
			  name: venafi-connection-secret-reader
			  namespace: venafi
			rules:
			- apiGroups: [""]
			  resources: ["secrets"]
			  verbs: ["get"]
			  resourceNames: ["accesstoken", "apikey"]
			---
			apiVersion: rbac.authorization.k8s.io/v1
			kind: RoleBinding
			metadata:
			  name: venafi-connection-secret-reader
			  namespace: venafi
			roleRef:
			  apiGroup: rbac.authorization.k8s.io
			  kind: Role
			  name: venafi-connection-secret-reader
			subjects:
			- kind: ServiceAccount
			  name: venafi-connection
			  namespace: venafi`))...)
		given = append(given, testutil.Parse(test.given)...)
		for _, obj := range given {
			require.NoError(t, kclient.Create(context.Background(), obj))
		}

		err = cl.PostDataReadingsWithOptions([]*api.DataReading{}, client.Options{ClusterName: "test cluster name"})
		if test.expectErr != "" {
			assert.EqualError(t, err, test.expectErr)
		}

		got := v1alpha1.VenafiConnection{}
		err = kclient.Get(context.Background(), types.NamespacedName{Name: "venafi-components", Namespace: "venafi"}, &got)
		require.NoError(t, err)
		require.Len(t, got.Status.Conditions, 1)
		assert.Equal(t, test.expectReadyCondMsg, got.Status.Conditions[0].Message)
	}
}
