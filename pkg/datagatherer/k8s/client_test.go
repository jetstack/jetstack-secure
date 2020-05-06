package k8s

import (
	"io/ioutil"
	"os"
	"testing"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
)

// These tests do not currently validate the created dynamic client uses the
// KUBECONFIG file that we create, however it _does_ exercise enough of the
// code path to show that the function is correctly selecting which file to
// load and returning it.

func TestNewDynamicClient_ExplicitKubeconfig(t *testing.T) {
	kc := createValidTestConfig()
	path := writeConfigToFile(t, kc)
	_, err := NewDynamicClient(path)
	if err != nil {
		t.Error("failed to create client: ", err)
	}
}

func TestNewDynamicClient_InferredKubeconfig(t *testing.T) {
	kc := createValidTestConfig()
	path := writeConfigToFile(t, kc)
	cleanupFn := temporarilySetEnv("KUBECONFIG", path)
	defer cleanupFn()
	_, err := NewDynamicClient("")
	if err != nil {
		t.Error("failed to create client: ", err)
	}
}

func TestNewDiscoveryClient_ExplicitKubeconfig(t *testing.T) {
	kc := createValidTestConfig()
	path := writeConfigToFile(t, kc)
	_, err := NewDiscoveryClient(path)
	if err != nil {
		t.Error("failed to create client: ", err)
	}
}

func TestNewDiscoveryClient_InferredKubeconfig(t *testing.T) {
	kc := createValidTestConfig()
	path := writeConfigToFile(t, kc)
	cleanupFn := temporarilySetEnv("KUBECONFIG", path)
	defer cleanupFn()
	_, err := NewDiscoveryClient("")
	if err != nil {
		t.Error("failed to create client: ", err)
	}
}

func writeConfigToFile(t *testing.T, cfg clientcmdapi.Config) string {
	f, err := ioutil.TempFile("", "testcase-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := clientcmdlatest.Codec.Encode(&cfg, f); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func createValidTestConfig() clientcmdapi.Config {
	const (
		server = "https://example.com:8080"
		token  = "the-token"
	)

	config := clientcmdapi.NewConfig()
	config.Clusters["clean"] = &clientcmdapi.Cluster{
		Server: server,
	}
	config.AuthInfos["clean"] = &clientcmdapi.AuthInfo{
		Token: token,
	}
	config.Contexts["clean"] = &clientcmdapi.Context{
		Cluster:  "clean",
		AuthInfo: "clean",
	}
	config.CurrentContext = "clean"

	return *config
}

func temporarilySetEnv(key, value string) func() {
	old := os.Getenv(key)
	os.Setenv(key, value)
	return func() {
		os.Setenv(key, old)
	}
}
