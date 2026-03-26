package client

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v4"
)

// parsePrivateKeyFromPEMFile reads and parses a PEM-encoded private key file.
func parsePrivateKeyFromPEMFile(privateKeyFilePath string) (crypto.PrivateKey, error) {
	pkBytes, err := os.ReadFile(privateKeyFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Venafi Cloud authentication private key %q: %s",
			privateKeyFilePath, err)
	}

	der, _ := pem.Decode(pkBytes)
	if der == nil {
		return nil, fmt.Errorf("while decoding the PEM-encoded private key %v, its content were: %s", privateKeyFilePath, string(pkBytes))
	}

	if key, err := x509.ParsePKCS1PrivateKey(der.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der.Bytes); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey, ed25519.PrivateKey:
			return key, nil
		default:
			return nil, fmt.Errorf("found unknown private key type in PKCS#8 wrapping: %T", key)
		}
	}
	if key, err := x509.ParseECPrivateKey(der.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("while parsing EC private: %w", err)
}

// parsePrivateKeyAndExtractSigningMethod parses a private key file and determines
// the appropriate JWT signing method based on the key type and size.
func parsePrivateKeyAndExtractSigningMethod(privateKeyFile string) (crypto.PrivateKey, jwt.SigningMethod, error) {
	privateKey, err := parsePrivateKeyFromPEMFile(privateKeyFile)
	if err != nil {
		return nil, nil, err
	}

	var signingMethod jwt.SigningMethod
	switch key := privateKey.(type) {
	case *rsa.PrivateKey:
		bitLen := key.N.BitLen()
		switch bitLen {
		case 2048:
			signingMethod = jwt.SigningMethodRS256
		case 3072:
			signingMethod = jwt.SigningMethodRS384
		case 4096:
			signingMethod = jwt.SigningMethodRS512
		default:
			signingMethod = jwt.SigningMethodRS256
		}

	case *ecdsa.PrivateKey:
		bitLen := key.Curve.Params().BitSize
		switch bitLen {
		case 256:
			signingMethod = jwt.SigningMethodES256
		case 384:
			signingMethod = jwt.SigningMethodES384
		case 521:
			signingMethod = jwt.SigningMethodES512
		default:
			signingMethod = jwt.SigningMethodES256
		}

	case ed25519.PrivateKey:
		signingMethod = jwt.SigningMethodEdDSA

	default:
		err = fmt.Errorf("unsupported private key type")
	}
	return privateKey, signingMethod, err
}
