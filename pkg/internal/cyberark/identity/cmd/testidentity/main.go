package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"k8s.io/klog/v2"

	"github.com/jetstack/preflight/pkg/internal/cyberark/identity"
	"github.com/jetstack/preflight/pkg/internal/cyberark/servicediscovery"
)

// This is a trivial CLI application for testing our identity client end-to-end.
// It's not intended for distribution; it simply allows us to run our client and check
// the login is successful.

const (
	subdomainFlag = "subdomain"
	usernameFlag  = "username"
	passwordEnv   = "TESTIDENTITY_PASSWORD"
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
	sdClient := servicediscovery.New(servicediscovery.WithIntegrationEndpoint())

	client, err := identity.NewWithDiscoveryClient(ctx, sdClient, subdomain)
	if err != nil {
		return err
	}

	err = client.LoginUsernamePassword(ctx, username, []byte(password))
	if err != nil {
		return err
	}

	return nil
}

func main() {
	defer klog.Flush()

	flagSet := flag.NewFlagSet("test", flag.ExitOnError)
	klog.InitFlags(flagSet)
	_ = flagSet.Parse([]string{"--v", "5"})

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
