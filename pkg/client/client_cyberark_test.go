package client

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/api"

	_ "k8s.io/klog/v2/ktesting/init"
)

// TestCyberArkClient_PostDataReadingsWithOptions_RealAPI demonstrates that the
// dataupload code works with the real inventory API.
//
// To enable verbose request logging:
//
//	go test ./pkg/internal/cyberark/dataupload/... \
//	  -v -count 1 -run TestPostDataReadingsWithOptionsWithRealAPI -args -testing.v 6
func TestCyberArkClient_PostDataReadingsWithOptions_RealAPI(t *testing.T) {
	platformDomain := os.Getenv("ARK_PLATFORM_DOMAIN")
	subdomain := os.Getenv("ARK_SUBDOMAIN")
	username := os.Getenv("ARK_USERNAME")
	secret := os.Getenv("ARK_SECRET")

	if platformDomain == "" || subdomain == "" || username == "" || secret == "" {
		t.Skip("Skipping because one of the following environment variables is unset or empty: ARK_PLATFORM_DOMAIN, ARK_SUBDOMAIN, ARK_USERNAME, ARK_SECRET")
		return
	}

	logger := ktesting.NewLogger(t, ktesting.DefaultConfig)
	ctx := klog.NewContext(t.Context(), logger)

	c, err := NewCyberArk()
	require.NoError(t, err)
	var readings []*api.DataReading
	err = c.PostDataReadingsWithOptions(ctx, readings, Options{})
	require.NoError(t, err)
}
