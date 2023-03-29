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

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	scheme "github.com/hazelcast/hazelcast-platform-operator/pkg/k8s/hazelcast/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ReplicatedMapsGetter has a method to return a ReplicatedMapInterface.
// A group's client should implement this interface.
type ReplicatedMapsGetter interface {
	ReplicatedMaps(namespace string) ReplicatedMapInterface
}

// ReplicatedMapInterface has methods to work with ReplicatedMap resources.
type ReplicatedMapInterface interface {
	Create(ctx context.Context, replicatedMap *v1alpha1.ReplicatedMap, opts v1.CreateOptions) (*v1alpha1.ReplicatedMap, error)
	Update(ctx context.Context, replicatedMap *v1alpha1.ReplicatedMap, opts v1.UpdateOptions) (*v1alpha1.ReplicatedMap, error)
	UpdateStatus(ctx context.Context, replicatedMap *v1alpha1.ReplicatedMap, opts v1.UpdateOptions) (*v1alpha1.ReplicatedMap, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ReplicatedMap, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ReplicatedMapList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ReplicatedMap, err error)
	ReplicatedMapExpansion
}

// replicatedMaps implements ReplicatedMapInterface
type replicatedMaps struct {
	client rest.Interface
	ns     string
}

// newReplicatedMaps returns a ReplicatedMaps
func newReplicatedMaps(c *HazelcastV1alpha1Client, namespace string) *replicatedMaps {
	return &replicatedMaps{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the replicatedMap, and returns the corresponding replicatedMap object, and an error if there is any.
func (c *replicatedMaps) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ReplicatedMap, err error) {
	result = &v1alpha1.ReplicatedMap{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("replicatedmaps").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ReplicatedMaps that match those selectors.
func (c *replicatedMaps) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ReplicatedMapList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ReplicatedMapList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("replicatedmaps").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested replicatedMaps.
func (c *replicatedMaps) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("replicatedmaps").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a replicatedMap and creates it.  Returns the server's representation of the replicatedMap, and an error, if there is any.
func (c *replicatedMaps) Create(ctx context.Context, replicatedMap *v1alpha1.ReplicatedMap, opts v1.CreateOptions) (result *v1alpha1.ReplicatedMap, err error) {
	result = &v1alpha1.ReplicatedMap{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("replicatedmaps").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(replicatedMap).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a replicatedMap and updates it. Returns the server's representation of the replicatedMap, and an error, if there is any.
func (c *replicatedMaps) Update(ctx context.Context, replicatedMap *v1alpha1.ReplicatedMap, opts v1.UpdateOptions) (result *v1alpha1.ReplicatedMap, err error) {
	result = &v1alpha1.ReplicatedMap{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("replicatedmaps").
		Name(replicatedMap.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(replicatedMap).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *replicatedMaps) UpdateStatus(ctx context.Context, replicatedMap *v1alpha1.ReplicatedMap, opts v1.UpdateOptions) (result *v1alpha1.ReplicatedMap, err error) {
	result = &v1alpha1.ReplicatedMap{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("replicatedmaps").
		Name(replicatedMap.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(replicatedMap).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the replicatedMap and deletes it. Returns an error if one occurs.
func (c *replicatedMaps) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("replicatedmaps").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *replicatedMaps) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("replicatedmaps").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched replicatedMap.
func (c *replicatedMaps) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ReplicatedMap, err error) {
	result = &v1alpha1.ReplicatedMap{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("replicatedmaps").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
