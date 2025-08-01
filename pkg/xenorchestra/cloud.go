/*
Copyright 2023 The Kubernetes Authors.

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

// Package xenorchestra is main CCM definition.
package xenorchestra

import (
	"context"
	"io"

	provider "github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/provider"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	// ProviderName is the name of the Xen Orchestra provider.
	ProviderName = provider.ProviderName

	// ServiceAccountName is the service account name used in kube-system namespace.
	ServiceAccountName = provider.ProviderName + "-cloud-controller-manager"
)

type cloud struct {
	client      *xoClient
	instancesV2 cloudprovider.InstancesV2

	ctx  context.Context //nolint:containedctx
	stop func()
}

func init() {
	cloudprovider.RegisterCloudProvider(provider.ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		cfg, err := readCloudConfig(config)
		if err != nil {
			klog.ErrorS(err, "failed to read config")

			return nil, err
		}

		return newCloud(&cfg)
	})
}

func newCloud(config *xoConfig) (cloudprovider.Interface, error) {
	client, err := newXOClient(config)
	if err != nil {
		return nil, err
	}

	instancesInterface := newInstances(client)

	return &cloud{
		client:      client,
		instancesV2: instancesInterface,
	}, nil
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping or run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	klog.InfoS("clientset initialized")

	ctx, cancel := context.WithCancel(context.Background())
	c.ctx = ctx
	c.stop = cancel

	err := c.client.CheckClient(ctx)
	if err != nil {
		klog.ErrorS(err, "failed to check Xen Orchestra client")
	}

	// Broadcast the upstream stop signal to all provider-level goroutines
	// watching the provider's context for cancellation.
	go func(provider *cloud) {
		<-stop
		klog.V(3).InfoS("received cloud provider termination signal")
		provider.stop()
	}(c)

	klog.InfoS("Xen Orchestra client initialized")
}

// LoadBalancer returns a balancer interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Instances returns an instances interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

// InstancesV2 is an implementation for instances and should only be implemented by external cloud providers.
// Implementing InstancesV2 is behaviorally identical to Instances but is optimized to significantly reduce
// API calls to the cloud provider when registering and syncing nodes.
// Also returns true if the interface is supported, false otherwise.
func (c *cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return c.instancesV2, c.instancesV2 != nil
}

// Zones returns a zones interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

// Clusters is not implemented.
func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes is not implemented.
func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (c *cloud) ProviderName() string {
	return provider.ProviderName
}

// HasClusterID is not implemented.
func (c *cloud) HasClusterID() bool {
	return true
}
