package k8sdynamic

import (
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/pmylund/go-cache"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2/ktesting"

	"github.com/jetstack/preflight/api"
)

func makeGatheredResource(obj runtime.Object, deletedAt api.Time) *api.GatheredResource {
	return &api.GatheredResource{
		Resource:  obj,
		DeletedAt: deletedAt,
	}
}

func TestOnAddCache(t *testing.T) {
	tcs := map[string]struct {
		inputObjects []runtime.Object
		eventObjects []runtime.Object
		eventFunc    func(log logr.Logger, old, obj any, dgCache *cache.Cache)
		expected     []*api.GatheredResource
	}{
		"add all objects": {
			inputObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("v1", "Service", "testservice", "testns", false),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
			},
			expected: []*api.GatheredResource{
				makeGatheredResource(getObject("foobar/v1", "Foo", "testfoo", "testns", false), api.Time{}),
				makeGatheredResource(getObject("v1", "Service", "testservice", "testns", false), api.Time{}),
				makeGatheredResource(getObject("foobar/v1", "NotFoo", "notfoo", "testns", false), api.Time{}),
			},
		},
		"delete all objects. All objects should have the deletedAt flag": {
			inputObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("v1", "Service", "testservice", "testns", false),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
			},
			// objects to delete
			eventObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("v1", "Service", "testservice", "testns", false),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
			},
			eventFunc: func(log logr.Logger, oldObj, newObj any, dgCache *cache.Cache) {
				onDelete(log, oldObj, dgCache)
			},
			expected: []*api.GatheredResource{
				makeGatheredResource(
					getObject("foobar/v1", "Foo", "testfoo", "testns", false),
					api.Time{Time: clock.now()},
				),
				makeGatheredResource(
					getObject("v1", "Service", "testservice", "testns", false),
					api.Time{Time: clock.now()},
				),
				makeGatheredResource(
					getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
					api.Time{Time: clock.now()},
				),
			},
		},
		"update all objects' namespace": {
			inputObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("v1", "Service", "testservice", "testns", false),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
			},
			// objects to update
			eventObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns1", false),
				getObject("v1", "Service", "testservice", "testns1", false),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns1", false),
			},
			eventFunc: onUpdate,
			expected: []*api.GatheredResource{
				makeGatheredResource(
					getObject("foobar/v1", "Foo", "testfoo", "testns1", false),
					api.Time{},
				),
				makeGatheredResource(
					getObject("v1", "Service", "testservice", "testns1", false),
					api.Time{},
				),
				makeGatheredResource(
					getObject("foobar/v1", "NotFoo", "notfoo", "testns1", false),
					api.Time{},
				),
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			log := ktesting.NewLogger(t, ktesting.NewConfig(ktesting.Verbosity(10)))
			dgCache := cache.New(5*time.Minute, 30*time.Second)
			// adding initial objetcs to the cache
			for _, obj := range tc.inputObjects {
				onAdd(log, obj, dgCache)
			}

			// Testing event founction on set of objects
			for _, obj := range tc.eventObjects {
				if tc.eventFunc != nil {
					tc.eventFunc(log, obj, obj, dgCache)
				}
			}

			// items back from the cache
			list := []*api.GatheredResource{}
			for _, item := range dgCache.Items() {
				cacheObject := item.Object.(*api.GatheredResource)
				list = append(list, cacheObject)
			}

			// sorting list of results by name
			sortGatheredResources(list)
			// sorting list of expected results by name
			sortGatheredResources(tc.expected)

			if len(list) != len(tc.expected) {
				t.Errorf("unexpected number of return items found. exp:%+v act:%+v", tc.expected, list)
			}

			require.Equal(t, tc.expected, list)
		})
	}
}

// TestNoneCache demonstrates that the cache helpers do not crash if passed a
// non-cachable object, but log an error with a reference to the object type.
func TestNoneCache(t *testing.T) {
	log := ktesting.NewLogger(t, ktesting.NewConfig(ktesting.Verbosity(10)))

	type notCachable struct{}
	onAdd(log, &notCachable{}, nil)
	onUpdate(log, &notCachable{}, nil, nil)
	onDelete(log, &notCachable{}, nil)
}
