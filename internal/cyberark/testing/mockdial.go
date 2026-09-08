package testing

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
)

// mockHostMu and mockHostAddr back a process-wide registry of fake hostname
// -> real host:port, shared across every package's Mock*Server helper. It
// has to be process-wide rather than scoped to one http.Client: tests freely
// reuse whichever mock's client is convenient to make a call against a
// *different* mock's server (e.g. keyfetch's test setup discards
// servicediscovery.MockDiscoveryServer's own client and instead reuses
// conjur.MockConjurExchangeServer's), so any mock client might end up being
// the one that has to dial any other mock's registered fake host.
var (
	mockHostMu   sync.Mutex
	mockHostAddr = map[string]string{}
)

// RegisterMockHost tells every WrapMockTransport-wrapped client to redirect
// dials for fakeHost to realHostPort instead. Used by servicediscovery's
// mock to embed another package's mock server address in a discovery
// response under a CyberArk-domain-looking hostname, since
// servicediscovery.DiscoverServices now allowlists both ARK_DISCOVERY_API
// itself and the hosts it returns (CP-25960, CP-26002).
func RegisterMockHost(fakeHost, realHostPort string) {
	mockHostMu.Lock()
	defer mockHostMu.Unlock()
	mockHostAddr[fakeHost] = realHostPort
}

// WrapMockTransport installs a DialContext on transport that redirects any
// RegisterMockHost-registered fake host to its real address, and disables
// TLS hostname verification, since a fake host never matches the httptest
// server's actual certificate SAN. Every package's Mock*Server should call
// this on its returned client's transport, so that client can reach a fake
// host registered by any other mock, regardless of which mock's client a
// test ends up reusing for a given call.
func WrapMockTransport(transport *http.Transport) {
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	transport.TLSClientConfig.InsecureSkipVerify = true
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if host, _, err := net.SplitHostPort(addr); err == nil {
			mockHostMu.Lock()
			realAddr, ok := mockHostAddr[host]
			mockHostMu.Unlock()
			if ok {
				addr = realAddr
			}
		}
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
}
