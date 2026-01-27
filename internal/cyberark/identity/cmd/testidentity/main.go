package main

import (
	"context"
	"crypto/x509"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/jetstack/venafi-connection-lib/http_client"
	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/internal/cyberark/identity"
	"github.com/jetstack/preflight/internal/cyberark/servicediscovery"
	"github.com/jetstack/preflight/pkg/version"
)

// This is a trivial CLI application for testing our identity client end-to-end.
// It's not intended for distribution; it simply allows us to run our client and check
// the login is successful.
//
// To test against a tenant on the integration platform, set:
// ARK_DISCOVERY_API=https://platform-discovery.integration-cyberark.cloud/
const (
	subdomainFlag = "subdomain"
	usernameFlag  = "username"
	passwordEnv   = "ARK_SECRET"
)

var (
	subdomain string
	username  string
)

func run(ctx context.Context) error {
	if subdomain == "" {
		return fmt.Errorf("no %s flag provided", subdomainFlag)
	}

	if username == "" {
		return fmt.Errorf("no %s flag provided", usernameFlag)
	}

	password := os.Getenv(passwordEnv)
	if password == "" {
		return fmt.Errorf("no password provided in %s", passwordEnv)
	}

	var rootCAs *x509.CertPool
	httpClient := http_client.NewDefaultClient(version.UserAgent(), rootCAs)

	sdClient := servicediscovery.New(httpClient, subdomain)
	services, _, err := sdClient.DiscoverServices(ctx)
	if err != nil {
		return fmt.Errorf("while performing service discovery: %s", err)
	}

	client := identity.New(httpClient, services.Identity.API, subdomain)

	err = client.LoginUsernamePassword(ctx, username, []byte(password))
	if err != nil {
		return fmt.Errorf("while performing login with username and password: %s", err)
	}

	return nil
}

func main() {
	defer klog.Flush()

	flagSet := flag.NewFlagSet("test", flag.ExitOnError)
	klog.InitFlags(flagSet)
	_ = flagSet.Parse([]string{"--v", "6"})

	logger := klog.Background()

	ctx := klog.NewContext(context.Background(), logger)
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	flag.StringVar(&subdomain, subdomainFlag, "cert-manager", "The subdomain to use for service discovery")
	flag.StringVar(&username, usernameFlag, "",
		fmt.Sprintf("Username to log in with. Password should be provided via %s envvar", passwordEnv),
	)

	flag.Parse()

	errCode := 0

	err := run(ctx)
	if err != nil {
		logger.Error(err, "execution failed")
		errCode = 1
	}

	klog.FlushAndExit(klog.ExitFlushTimeout, errCode)
}
