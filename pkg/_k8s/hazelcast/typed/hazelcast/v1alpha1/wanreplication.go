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

// WanReplicationsGetter has a method to return a WanReplicationInterface.
// A group's client should implement this interface.
type WanReplicationsGetter interface {
	WanReplications(namespace string) WanReplicationInterface
}

// WanReplicationInterface has methods to work with WanReplication resources.
type WanReplicationInterface interface {
	Create(ctx context.Context, wanReplication *v1alpha1.WanReplication, opts v1.CreateOptions) (*v1alpha1.WanReplication, error)
	Update(ctx context.Context, wanReplication *v1alpha1.WanReplication, opts v1.UpdateOptions) (*v1alpha1.WanReplication, error)
	UpdateStatus(ctx context.Context, wanReplication *v1alpha1.WanReplication, opts v1.UpdateOptions) (*v1alpha1.WanReplication, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.WanReplication, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.WanReplicationList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.WanReplication, err error)
	WanReplicationExpansion
}

// wanReplications implements WanReplicationInterface
type wanReplications struct {
	client rest.Interface
	ns     string
}

// newWanReplications returns a WanReplications
func newWanReplications(c *HazelcastV1alpha1Client, namespace string) *wanReplications {
	return &wanReplications{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the wanReplication, and returns the corresponding wanReplication object, and an error if there is any.
func (c *wanReplications) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.WanReplication, err error) {
	result = &v1alpha1.WanReplication{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("wanreplications").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of WanReplications that match those selectors.
func (c *wanReplications) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.WanReplicationList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.WanReplicationList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("wanreplications").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested wanReplications.
func (c *wanReplications) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("wanreplications").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a wanReplication and creates it.  Returns the server's representation of the wanReplication, and an error, if there is any.
func (c *wanReplications) Create(ctx context.Context, wanReplication *v1alpha1.WanReplication, opts v1.CreateOptions) (result *v1alpha1.WanReplication, err error) {
	result = &v1alpha1.WanReplication{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("wanreplications").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(wanReplication).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a wanReplication and updates it. Returns the server's representation of the wanReplication, and an error, if there is any.
func (c *wanReplications) Update(ctx context.Context, wanReplication *v1alpha1.WanReplication, opts v1.UpdateOptions) (result *v1alpha1.WanReplication, err error) {
	result = &v1alpha1.WanReplication{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("wanreplications").
		Name(wanReplication.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(wanReplication).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *wanReplications) UpdateStatus(ctx context.Context, wanReplication *v1alpha1.WanReplication, opts v1.UpdateOptions) (result *v1alpha1.WanReplication, err error) {
	result = &v1alpha1.WanReplication{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("wanreplications").
		Name(wanReplication.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(wanReplication).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the wanReplication and deletes it. Returns an error if one occurs.
func (c *wanReplications) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("wanreplications").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *wanReplications) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("wanreplications").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched wanReplication.
func (c *wanReplications) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.WanReplication, err error) {
	result = &v1alpha1.WanReplication{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("wanreplications").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
