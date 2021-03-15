package k8s

import (
	"time"

	"github.com/jetstack/preflight/api"
	"github.com/pmylund/go-cache"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type timeInterface interface {
	now() time.Time
}

var clock timeInterface = &realTime{}

type realTime struct {
}

func (*realTime) now() time.Time {
	return time.Now()
}

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
