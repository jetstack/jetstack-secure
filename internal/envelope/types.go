package envelope

import (
	"context"
	"encoding/json"
)

// EncryptedData represents encrypted data along with metadata about the encryption type.
type EncryptedData struct {
	// Data contains the encrypted payload
	Data []byte `json:"data"`
	// Type indicates the encryption format (e.g., "JWE-RSA")
	Type string `json:"type"`
}

// ToMap converts the EncryptedData struct to a map representation. Since we store data as an "_encryptedData" field in
// a Kubernetes unstructured object, passing a raw struct would cause a panic due to the behaviour of
// https://pkg.go.dev/k8s.io/apimachinery/pkg/runtime#DeepCopyJSONValue
// Passing a map to unstructured.SetNestedField avoids this issue.
func (ed *EncryptedData) ToMap() map[string]any {
	marshalled, err := json.Marshal(ed)
	if err != nil {
		return nil
	}

	var out map[string]any

	err = json.Unmarshal(marshalled, &out)
	if err != nil {
		return nil
	}

	return out
}

// Encryptor performs envelope encryption on arbitrary data.
type Encryptor interface {
	// Encrypt encrypts data using envelope encryption, returning an EncryptedData struct
	// containing the encrypted payload and encryption type metadata.
	Encrypt(ctx context.Context, data []byte) (*EncryptedData, error)
}
