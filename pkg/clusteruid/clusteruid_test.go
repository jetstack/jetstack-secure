package clusteruid

import (
	"testing"

	"k8s.io/client-go/kubernetes/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetClusterUID(t *testing.T) {
	client := fake.NewSimpleClientset()

	mockUID := "12345678-1234-5678-1234-567812345678"

	kubeSystemNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
			UID:  types.UID(mockUID),
		},
	}

	_, err := client.CoreV1().Namespaces().Create(t.Context(), kubeSystemNS, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create kube-system namespace with fake client: %v", err)
	}

	ctx, err := GetClusterUID(t.Context(), client)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	uid := ClusterUIDFromContext(ctx)

	if uid != mockUID {
		t.Fatalf("expected to get uid=%v, but got uid=%v", mockUID, uid)
	}
}
