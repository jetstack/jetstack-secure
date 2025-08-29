# README

Data captured from a cert-manager E2E test cluster.

```bash
cd cert-manager
make e2e-setup
```

```bash
cd jetstack-secure
go run . agent \
    --log-level 6 \
    --one-shot \
    --agent-config-file pkg/client/testdata/example-1/agent.yaml \
    --output-path pkg/client/testdata/example-1/datareadings.json
gzip pkg/internal/cyberark/dataupload/testdata/example-1/datareadings.json
```


To recreate the golden output file:

```bash
UPDATE_GOLDEN_FILES=true go test ./pkg/client/... -run TestConvertDataReadingsToCyberarkSnapshot
```
