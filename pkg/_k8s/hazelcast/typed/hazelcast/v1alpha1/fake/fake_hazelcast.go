/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeHazelcasts implements HazelcastInterface
type FakeHazelcasts struct {
	Fake *FakeHazelcastV1alpha1
	ns   string
}

var hazelcastsResource = schema.GroupVersionResource{Group: "hazelcast", Version: "v1alpha1", Resource: "hazelcasts"}

var hazelcastsKind = schema.GroupVersionKind{Group: "hazelcast", Version: "v1alpha1", Kind: "Hazelcast"}

// Get takes name of the hazelcast, and returns the corresponding hazelcast object, and an error if there is any.
func (c *FakeHazelcasts) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.Hazelcast, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(hazelcastsResource, c.ns, name), &v1alpha1.Hazelcast{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Hazelcast), err
}

// List takes label and field selectors, and returns the list of Hazelcasts that match those selectors.
func (c *FakeHazelcasts) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.HazelcastList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(hazelcastsResource, hazelcastsKind, c.ns, opts), &v1alpha1.HazelcastList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.HazelcastList{ListMeta: obj.(*v1alpha1.HazelcastList).ListMeta}
	for _, item := range obj.(*v1alpha1.HazelcastList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested hazelcasts.
func (c *FakeHazelcasts) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(hazelcastsResource, c.ns, opts))

}

// Create takes the representation of a hazelcast and creates it.  Returns the server's representation of the hazelcast, and an error, if there is any.
func (c *FakeHazelcasts) Create(ctx context.Context, hazelcast *v1alpha1.Hazelcast, opts v1.CreateOptions) (result *v1alpha1.Hazelcast, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(hazelcastsResource, c.ns, hazelcast), &v1alpha1.Hazelcast{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Hazelcast), err
}

// Update takes the representation of a hazelcast and updates it. Returns the server's representation of the hazelcast, and an error, if there is any.
func (c *FakeHazelcasts) Update(ctx context.Context, hazelcast *v1alpha1.Hazelcast, opts v1.UpdateOptions) (result *v1alpha1.Hazelcast, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(hazelcastsResource, c.ns, hazelcast), &v1alpha1.Hazelcast{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Hazelcast), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeHazelcasts) UpdateStatus(ctx context.Context, hazelcast *v1alpha1.Hazelcast, opts v1.UpdateOptions) (*v1alpha1.Hazelcast, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(hazelcastsResource, "status", c.ns, hazelcast), &v1alpha1.Hazelcast{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Hazelcast), err
}

// Delete takes name of the hazelcast and deletes it. Returns an error if one occurs.
func (c *FakeHazelcasts) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(hazelcastsResource, c.ns, name, opts), &v1alpha1.Hazelcast{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeHazelcasts) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(hazelcastsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.HazelcastList{})
	return err
}

// Patch applies the patch and returns the patched hazelcast.
func (c *FakeHazelcasts) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Hazelcast, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(hazelcastsResource, c.ns, name, pt, data, subresources...), &v1alpha1.Hazelcast{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Hazelcast), err
}
