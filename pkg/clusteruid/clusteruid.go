package clusteruid

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// clusterUIDKey is the context key for storing the cluster UID
type clusterUIDKey struct{}

// GetClusterUID retrieves the UID of the kube-system namespace using the given Kubernetes clientset.
// This UID can be used as a unique identifier for the Kubernetes cluster.
// The UID is stored in the given context for later retrieval; use ClusterUIDFromContext to get it.
func GetClusterUID(ctx context.Context, clientset kubernetes.Interface) (context.Context, error) {
	namespace, err := clientset.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{})
	if err != nil {
		return ctx, err
	}

	ctx = withClusterUID(ctx, string(namespace.ObjectMeta.UID))
	return ctx, nil
}

// ClusterUIDFromContext retrieves the cluster UID from the context.
// Panics if the value is not found or if the value is not a string.
func ClusterUIDFromContext(ctx context.Context) string {
	value := ctx.Value(clusterUIDKey{})
	if value == nil {
		panic("cluster UID not found in context")
	}

	uid, ok := value.(string)
	if !ok {
		panic("cluster UID in context is not a string")
	}

	return uid
}

// withClusterUID adds the given cluster UID to the context
func withClusterUID(ctx context.Context, clusterUID string) context.Context {
	return context.WithValue(ctx, clusterUIDKey{}, clusterUID)
}
