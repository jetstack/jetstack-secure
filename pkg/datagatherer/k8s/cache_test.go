package k8s

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/d4l3k/messagediff"
	"github.com/jetstack/preflight/api"
	"github.com/pmylund/go-cache"
	"k8s.io/apimachinery/pkg/runtime"
)

func makeGatheredResource(obj runtime.Object, data *api.GatheredResourceMetadata) *api.GatheredResource {
	return &api.GatheredResource{
		Resource:   obj,
		Properties: data,
	}
}

func TestOnAddCache(t *testing.T) {
	tcs := map[string]struct {
		inputObjects []runtime.Object
		eventObjects []runtime.Object
		eventFunc    func(old, obj interface{}, dgCache *cache.Cache)
		expected     []*api.GatheredResource
	}{
		"add all objects": {
			inputObjects: []runtime.Object{
				getObject("foobar/v1", "Foo", "testfoo", "testns", false),
				getObject("v1", "Service", "testservice", "testns", false),
				getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
			},
			expected: []*api.GatheredResource{
				makeGatheredResource(getObject("foobar/v1", "Foo", "testfoo", "testns", false), &api.GatheredResourceMetadata{}),
				makeGatheredResource(getObject("v1", "Service", "testservice", "testns", false), &api.GatheredResourceMetadata{}),
				makeGatheredResource(getObject("foobar/v1", "NotFoo", "notfoo", "testns", false), &api.GatheredResourceMetadata{}),
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
			eventFunc: func(old, new interface{}, dgCache *cache.Cache) { onDelete(old, dgCache) },
			expected: []*api.GatheredResource{
				makeGatheredResource(getObject("foobar/v1", "Foo", "testfoo", "testns", false),
					&api.GatheredResourceMetadata{
						DeletedAt: &api.Time{Time: clock.now()},
					}),
				makeGatheredResource(getObject("v1", "Service", "testservice", "testns", false),
					&api.GatheredResourceMetadata{
						DeletedAt: &api.Time{Time: clock.now()},
					}),
				makeGatheredResource(getObject("foobar/v1", "NotFoo", "notfoo", "testns", false),
					&api.GatheredResourceMetadata{
						DeletedAt: &api.Time{Time: clock.now()},
					}),
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
				makeGatheredResource(getObject("foobar/v1", "Foo", "testfoo", "testns1", false), &api.GatheredResourceMetadata{}),
				makeGatheredResource(getObject("v1", "Service", "testservice", "testns1", false), &api.GatheredResourceMetadata{}),
				makeGatheredResource(getObject("foobar/v1", "NotFoo", "notfoo", "testns1", false), &api.GatheredResourceMetadata{}),
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			dgCache := cache.New(5*time.Minute, 30*time.Second)
			// adding initial objetcs to the cache
			for _, obj := range tc.inputObjects {
				onAdd(obj, dgCache)
			}

			// Testing event founction on set of objects
			for _, obj := range tc.eventObjects {
				if tc.eventFunc != nil {
					tc.eventFunc(obj, obj, dgCache)
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

			if diff, equal := messagediff.PrettyDiff(tc.expected, list); !equal {
				t.Errorf("\n%s", diff)
				expectedJSON, _ := json.MarshalIndent(tc.expected, "", "  ")
				gotJSON, _ := json.MarshalIndent(list, "", "  ")
				t.Fatalf("unexpected JSON: \ngot \n%s\nwant\n%s", string(gotJSON), expectedJSON)
			}
		})
	}
}
