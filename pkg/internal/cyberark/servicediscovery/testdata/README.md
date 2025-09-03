# Test data for CyberArk Discovery

All data in this folder is derived from an unauthenticated endpoint accessible from the public Internet.

To get the original data:

NOTE: This API is not implemented yet as of 02.09.2025 but is expected to be finalised by end of PI3 2025.
```bash
curl -fsSL "${ARK_DISCOVERY_API}/api/tenant-discovery/public?bySubdomain=${ARK_SUBDOMAIN}" | jq
```

Then replace `identity_administration.api` with `{{ .Identity.API }}` and
`discoverycontext.api` with `{{ .DiscoveryContext.API }}`. Those Go template
fields will be substituted in the tests.
