package k8s

import (
	"time"

	"github.com/pmylund/go-cache"
	"k8s.io/apimachinery/pkg/types"

	"github.com/jetstack/preflight/api"
	"github.com/jetstack/preflight/pkg/logs"
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

// onAdd handles the informer creation events, adding the created runtime.Object
// to the data gatherer's cache. The cache key is the uid of the object
func onAdd(obj interface{}, dgCache *cache.Cache) {
	item, ok := obj.(cacheResource)
	if ok {
		cacheObject := &api.GatheredResource{
			Resource: obj,
		}
		dgCache.Set(string(item.GetUID()), cacheObject, cache.DefaultExpiration)
		return
	}
	logs.Log.Printf("could not %q resource to the cache, missing metadata/uid field", "add")

}

// onUpdate handles the informer update events, replacing the old object with the new one
// if it's present in the data gatherer's cache, (if the object isn't present, it gets added).
// The cache key is the uid of the object
func onUpdate(old, new interface{}, dgCache *cache.Cache) {
	item, ok := old.(cacheResource)
	if ok {
		cacheObject := updateCacheGatheredResource(string(item.GetUID()), new, dgCache)
		dgCache.Set(string(item.GetUID()), cacheObject, cache.DefaultExpiration)
		return
	}

	logs.Log.Printf("could not %q resource to the cache, missing metadata/uid field", "update")
}

// onDelete handles the informer deletion events, updating the object's properties with the deletion
// time of the object (but not removing the object from the cache).
// The cache key is the uid of the object
func onDelete(obj interface{}, dgCache *cache.Cache) {
	item, ok := obj.(cacheResource)
	if ok {
		cacheObject := updateCacheGatheredResource(string(item.GetUID()), obj, dgCache)
		cacheObject.DeletedAt = api.Time{Time: clock.now()}
		dgCache.Set(string(item.GetUID()), cacheObject, cache.DefaultExpiration)
		return
	}
	logs.Log.Printf("could not %q resource to the cache, missing metadata/uid field", "delete")
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
