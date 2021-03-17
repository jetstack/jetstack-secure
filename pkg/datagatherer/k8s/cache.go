package k8s

import (
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/pmylund/go-cache"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// onAdd handles the informer creation events, adding the created runtime.Object
// to the data gatherer's cache. The cache key is the uid of the object
func onAdd(obj interface{}, dgCache *cache.Cache) {
	item := obj.(*unstructured.Unstructured)
	if metadata, ok := item.Object["metadata"]; ok {
		data := metadata.(map[string]interface{})
		if uid, ok := data["uid"]; ok {
			cacheObject := &api.GatheredResource{
				Resource:   obj,
				Properties: &api.GatheredResourceMetadata{},
			}
			dgCache.Set(uid.(string), cacheObject, cache.DefaultExpiration)
		}
	}
}

// onUpdate handles the informer update events, replacing the old object with the new one
// if it's present in the data gatherer's cache, (if the object isn't present, it gets added).
// The cache key is the uid of the object
func onUpdate(old, new interface{}, dgCache *cache.Cache) {
	item := old.(*unstructured.Unstructured)
	if metadata, ok := item.Object["metadata"]; ok {
		data := metadata.(map[string]interface{})
		if uid, ok := data["uid"]; ok {
			cacheObject := updateCacheGatheredResource(uid.(string), new, dgCache)
			dgCache.Set(uid.(string), cacheObject, cache.DefaultExpiration)
		}
	}
}

// onDelete handles the informer deletion events, updating the object's properties with the deletion
// time of the object (but not removing the object from the cache).
// The cache key is the uid of the object
func onDelete(obj interface{}, dgCache *cache.Cache) {
	item := obj.(*unstructured.Unstructured)
	if metadata, ok := item.Object["metadata"]; ok {
		data := metadata.(map[string]interface{})
		if uid, ok := data["uid"]; ok {
			cacheObject := updateCacheGatheredResource(uid.(string), obj, dgCache)
			cacheObject.Properties.DeletedAt = &api.Time{Time: clock.now()}
			dgCache.Set(uid.(string), cacheObject, cache.DefaultExpiration)
		}
	}
}

// creates a new updated instance of a cache object, with the resource
// argument. If the object is present in the cache it fetches the object's
// properties.
func updateCacheGatheredResource(cacheKey string, resource interface{},
	dgCache *cache.Cache) *api.GatheredResource {
	// updated cache object
	cacheObject := &api.GatheredResource{
		Resource:   resource,
		Properties: &api.GatheredResourceMetadata{},
	}
	// update the object's properties, if it's already in the cache
	if o, ok := dgCache.Get(cacheKey); ok {
		cachedProperties := o.(*api.GatheredResource).Properties
		if cachedProperties != nil {
			cacheObject.Properties = cachedProperties
		}
	}
	return cacheObject
}
