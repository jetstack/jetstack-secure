package k8s

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/pmylund/go-cache"
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

func logCacheUpdateFailure(log logr.Logger, obj any, operation string) {
	// We use WithCallStackHelper to ensure the correct caller line numbers in the log messages
	helper, log := log.WithCallStackHelper()
	helper()
	err := fmt.Errorf("not a cacheResource type: %T missing metadata/uid field", obj)
	log.Error(err, "Cache update failure", "operation", operation)
}

// onAdd handles the informer creation events, adding the created runtime.Object
// to the data gatherer's cache. The cache key is the uid of the object
func onAdd(log logr.Logger, obj any, dgCache *cache.Cache) {
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
func onUpdate(log logr.Logger, oldObj, newObj any, dgCache *cache.Cache) {
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
func onDelete(log logr.Logger, obj any, dgCache *cache.Cache) {
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
func updateCacheGatheredResource(cacheKey string, resource any, dgCache *cache.Cache) *api.GatheredResource {
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
