package k8s

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pmylund/go-cache"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/jetstack/preflight/api"
)

// time interface, this is used to fetch the current time
// whenever a k8s resource is deleted
type timeInterface interface {
	now() time.Time
}

var clock timeInterface = &realTime{}

type realTime struct {
}

func (*realTime) now() time.Time {
	return time.Now()
}

type cacheResource interface {
	GetUID() types.UID
	GetNamespace() string
}

func logCacheUpdateFailure(log logr.Logger, obj interface{}, operation string) {
	// We use WithCallStackHelper to ensure the correct caller line numbers in the log messages
	helper, log := log.WithCallStackHelper()
	helper()
	err := fmt.Errorf("not a cacheResource type: %T missing metadata/uid field", obj)
	log.Error(err, "Cache update failure", "operation", operation)
}

// cacheFilterFunction is a function that can be used to filter out objects
// that should not be added to the cache. If the function returns true, the
// object is filtered out.
type cacheFilterFunction func(logr.Logger, interface{}) bool

// excludeTLSSecretsWithoutClientCert filters out all TLS secrets that do not
// contain a client certificate in the `tls.crt` key.
// Secrets are obtained by a DynamicClient, so they have type
// *unstructured.Unstructured.
func excludeTLSSecretsWithoutClientCert(log logr.Logger, obj interface{}) bool {
	// Fast path: type assertion and kind/type checks
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		log.V(4).Info("Object is not a Unstructured", "type", fmt.Sprintf("%T", obj))
		return false
	}
	if unstructuredObj.GetKind() != "Secret" || unstructuredObj.GetAPIVersion() != "v1" {
		return false
	}

	log = log.WithValues("namespace", unstructuredObj.GetNamespace(), "name", unstructuredObj.GetName())

	secretType, found, err := unstructured.NestedString(unstructuredObj.Object, "type")
	if err != nil || !found || secretType != string(corev1.SecretTypeTLS) {
		log.V(4).Info("Object is not a TLS Secret", "type", secretType)
		return false
	}

	// Directly extract tls.crt from unstructured data (avoid conversion if possible)
	dataMap, found, err := unstructured.NestedMap(unstructuredObj.Object, "data")
	if err != nil || !found {
		log.V(4).Info("Secret data missing or not a map")
		return true
	}
	tlsCrtRaw, found := dataMap[corev1.TLSCertKey]
	if !found {
		log.V(4).Info("TLS Secret does not contain tls.crt key")
		return true
	}

	// Decode base64 if necessary (K8s secrets store data as base64-encoded strings)
	var tlsCrtBytes []byte
	switch v := tlsCrtRaw.(type) {
	case string:
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			log.V(4).Info("Failed to decode tls.crt base64", "error", err.Error())
			return true
		}
		tlsCrtBytes = decoded
	case []byte:
		tlsCrtBytes = v
	default:
		log.V(4).Info("tls.crt is not a string or byte slice", "type", fmt.Sprintf("%T", v))
		return true
	}

	// Parse PEM certificate chain
	certs, err := parsePEMCertificateChain(tlsCrtBytes)
	if err != nil || len(certs) == 0 {
		log.V(4).Info("Failed to parse tls.crt as PEM encoded X.509 certificate chain", "error", err.Error())
		return true
	}

	// Check if the leaf certificate is a client certificate
	if isClientCertificate(certs[0]) {
		log.V(4).Info("TLS Secret contains a client certificate")
		return false
	}

	log.V(4).Info("TLS Secret does not contain a client certificate")
	return true
}

// isClientCertificate checks if the given certificate is a client certificate
// by checking if it has the ClientAuth EKU.
func isClientCertificate(cert *x509.Certificate) bool {
	if cert == nil {
		return false
	}
	// Check if the certificate has the ClientAuth EKU
	for _, eku := range cert.ExtKeyUsage {
		if eku == x509.ExtKeyUsageClientAuth {
			return true
		}
	}
	return false
}

// parsePEMCertificateChain parses a PEM encoded certificate chain and returns
// a slice of x509.Certificate pointers. It returns an error if the data cannot
// be parsed as a certificate chain.
// The supplied data can contain multiple PEM blocks, the function will parse
// all of them and return a slice of certificates.
func parsePEMCertificateChain(data []byte) ([]*x509.Certificate, error) {
	// Parse the PEM encoded certificate chain
	var certs []*x509.Certificate
	var block *pem.Block
	rest := data
	for {
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" || len(block.Bytes) == 0 {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}
		certs = append(certs, cert)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("no certificates found")
	}
	return certs, nil
}

// onAdd handles the informer creation events, adding the created runtime.Object
// to the data gatherer's cache. The cache key is the uid of the object
// The object is wrapped in a GatheredResource struct.
// If the object is already present in the cache, it gets replaced.
// The cache key is the uid of the object
// The supplied filter functions can be used to filter out objects that
// should not be added to the cache.
// If multiple filter functions are supplied, the object is filtered out
// if any of the filter functions returns true.
func onAdd(log logr.Logger, obj interface{}, dgCache *cache.Cache, filters ...cacheFilterFunction) {
	for _, filter := range filters {
		if filter != nil && filter(log, obj) {
			return
		}
	}

	item, ok := obj.(cacheResource)
	if ok {
		cacheObject := &api.GatheredResource{
			Resource: obj,
		}
		dgCache.Set(string(item.GetUID()), cacheObject, cache.DefaultExpiration)
		return
	}
	logCacheUpdateFailure(log, obj, "add")
}

// onUpdate handles the informer update events, replacing the old object with the new one
// if it's present in the data gatherer's cache, (if the object isn't present, it gets added).
// The cache key is the uid of the object
func onUpdate(log logr.Logger, oldObj, newObj interface{}, dgCache *cache.Cache) {
	item, ok := oldObj.(cacheResource)
	if ok {
		cacheObject := updateCacheGatheredResource(string(item.GetUID()), newObj, dgCache)
		dgCache.Set(string(item.GetUID()), cacheObject, cache.DefaultExpiration)
		return
	}
	logCacheUpdateFailure(log, oldObj, "update")
}

// onDelete handles the informer deletion events, updating the object's properties with the deletion
// time of the object (but not removing the object from the cache).
// The cache key is the uid of the object
func onDelete(log logr.Logger, obj interface{}, dgCache *cache.Cache) {
	item, ok := obj.(cacheResource)
	if ok {
		cacheObject := updateCacheGatheredResource(string(item.GetUID()), obj, dgCache)
		cacheObject.DeletedAt = api.Time{Time: clock.now()}
		dgCache.Set(string(item.GetUID()), cacheObject, cache.DefaultExpiration)
		return
	}
	logCacheUpdateFailure(log, obj, "delete")
}

// creates a new updated instance of a cache object, with the resource
// argument. If the object is present in the cache it fetches the object's
// properties.
func updateCacheGatheredResource(cacheKey string, resource interface{}, dgCache *cache.Cache) *api.GatheredResource {
	// updated cache object
	cacheObject := &api.GatheredResource{
		Resource: resource,
	}
	// update the object's properties, if it's already in the cache
	if o, ok := dgCache.Get(cacheKey); ok {
		deletedAt := o.(*api.GatheredResource).DeletedAt
		if deletedAt.IsZero() && !deletedAt.IsZero() {
			cacheObject.DeletedAt = deletedAt
		}
	}
	return cacheObject
}
