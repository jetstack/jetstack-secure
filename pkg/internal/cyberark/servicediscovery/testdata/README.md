# Test data for CyberArk Discovery

All data in this folder is derived from an unauthenticated endpoint accessible from the public Internet.

To get the original data:

```bash
curl -fsSL "${ARK_DISCOVERY_API}/services/subdomain/${ARK_SUBDOMAIN}" | jq
```

Then replace `identity_administration.api` with `{{ .Identity.API }}` and
`discoverycontext.api` with `{{ .DiscoveryContext.API }}`. Those Go template
fields will be substituted in the tests.
