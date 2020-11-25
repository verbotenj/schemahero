/*
Copyright 2020 Replicated, Inc.

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

package v1alpha4

import (
	"context"
	"time"

	v1alpha4 "github.com/schemahero/schemahero/pkg/apis/schemas/v1alpha4"
	scheme "github.com/schemahero/schemahero/pkg/client/schemaheroclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// MigrationsGetter has a method to return a MigrationInterface.
// A group's client should implement this interface.
type MigrationsGetter interface {
	Migrations(namespace string) MigrationInterface
}

// MigrationInterface has methods to work with Migration resources.
type MigrationInterface interface {
	Create(ctx context.Context, migration *v1alpha4.Migration, opts v1.CreateOptions) (*v1alpha4.Migration, error)
	Update(ctx context.Context, migration *v1alpha4.Migration, opts v1.UpdateOptions) (*v1alpha4.Migration, error)
	UpdateStatus(ctx context.Context, migration *v1alpha4.Migration, opts v1.UpdateOptions) (*v1alpha4.Migration, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha4.Migration, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha4.MigrationList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha4.Migration, err error)
	MigrationExpansion
}

// migrations implements MigrationInterface
type migrations struct {
	client rest.Interface
	ns     string
}

// newMigrations returns a Migrations
func newMigrations(c *Schemasv1alpha4Client, namespace string) *migrations {
	return &migrations{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the migration, and returns the corresponding migration object, and an error if there is any.
func (c *migrations) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha4.Migration, err error) {
	result = &v1alpha4.Migration{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("migrations").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Migrations that match those selectors.
func (c *migrations) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha4.MigrationList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha4.MigrationList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("migrations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested migrations.
func (c *migrations) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("migrations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a migration and creates it.  Returns the server's representation of the migration, and an error, if there is any.
func (c *migrations) Create(ctx context.Context, migration *v1alpha4.Migration, opts v1.CreateOptions) (result *v1alpha4.Migration, err error) {
	result = &v1alpha4.Migration{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("migrations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(migration).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a migration and updates it. Returns the server's representation of the migration, and an error, if there is any.
func (c *migrations) Update(ctx context.Context, migration *v1alpha4.Migration, opts v1.UpdateOptions) (result *v1alpha4.Migration, err error) {
	result = &v1alpha4.Migration{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("migrations").
		Name(migration.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(migration).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *migrations) UpdateStatus(ctx context.Context, migration *v1alpha4.Migration, opts v1.UpdateOptions) (result *v1alpha4.Migration, err error) {
	result = &v1alpha4.Migration{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("migrations").
		Name(migration.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(migration).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the migration and deletes it. Returns an error if one occurs.
func (c *migrations) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("migrations").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *migrations) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("migrations").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched migration.
func (c *migrations) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha4.Migration, err error) {
	result = &v1alpha4.Migration{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("migrations").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}