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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	xok8s "github.com/vatesfr/xenorchestra-k8s-common"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	// ProviderName is the name of the Xen Orchestra provider.
	ProviderName = xok8s.ProviderName

	cloudControllerManagerClientName = "xenorchestra-cloud-controller-manager"
	componentKind                    = "Component"
	podKind                          = "Pod"
	podNameEnv                       = "POD_NAME"
	podNamespaceEnv                  = "POD_NAMESPACE"
	podUIDEnv                        = "POD_UID"
	eventActionCheckClient           = "CheckClient"
	eventReasonFailedToCheckClient   = "FailedToCheckClient"
)

type cloud struct {
	client      *xok8s.XoClient
	instancesV2 cloudprovider.InstancesV2

	ctx  context.Context //nolint:containedctx
	stop func()
}

func init() {
	cloudprovider.RegisterCloudProvider(xok8s.ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		if config != nil {
			cfg, err := xok8s.ReadCloudConfig(config)
			if err != nil {
				klog.ErrorS(err, "failed to read config")

				return nil, err
			}

			return newCloud(&cfg)
		}

		cfg, err := xok8s.LoadXOConfigFromEnv()
		if err != nil {
			klog.ErrorS(err, "failed to read config from environment")

			return nil, err
		}

		return newCloud(&cfg)
	})
}

func newCloud(config *xok8s.XoConfig) (cloudprovider.Interface, error) {
	client, err := xok8s.NewXOClient(config)
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

	// #nosec G118 -- The cancel function is stored in the cloud struct and called when the provider receives a stop signal
	ctx, cancel := context.WithCancel(context.Background())
	c.ctx = ctx
	c.stop = cancel

	err := c.client.CheckClient(ctx)
	if err != nil {
		klog.ErrorS(err, "failed to check Xen Orchestra client")
		kubeClient := clientBuilder.ClientOrDie(cloudControllerManagerClientName)
		if eventErr := recordCloudProviderInitializationFailure(ctx, kubeClient, err); eventErr != nil {
			klog.ErrorS(eventErr, "failed to record Xen Orchestra client check failure event")
		}
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
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

func recordCloudProviderInitializationFailure(ctx context.Context, kubeClient clientset.Interface, err error) error {
	eventTime := metav1.MicroTime{Time: time.Now()}
	regarding := cloudProviderEventObjectReference()
	event := &eventsv1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cloudProviderEventName(regarding),
			Namespace: cloudProviderEventNamespace(),
		},
		EventTime:           eventTime,
		ReportingController: ProviderName,
		ReportingInstance:   cloudProviderEventReportingInstance(),
		Action:              eventActionCheckClient,
		Reason:              eventReasonFailedToCheckClient,
		Regarding:           regarding,
		Note:                fmt.Sprintf("Failed to check Xen Orchestra client: %v", err),
		Type:                corev1.EventTypeWarning,
	}

	eventsClient := kubeClient.EventsV1().Events(cloudProviderEventNamespace())
	if _, createErr := eventsClient.Create(ctx, event, metav1.CreateOptions{}); createErr != nil {
		if !apierrors.IsAlreadyExists(createErr) {
			return createErr
		}

		existingEvent, getErr := eventsClient.Get(ctx, event.Name, metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}

		event.ResourceVersion = existingEvent.ResourceVersion
		event.EventTime = existingEvent.EventTime
		event.Series = &eventsv1.EventSeries{
			Count:            2,
			LastObservedTime: eventTime,
		}
		if existingEvent.Series != nil {
			event.Series.Count = existingEvent.Series.Count + 1
		}

		_, updateErr := eventsClient.Update(ctx, event, metav1.UpdateOptions{})
		return updateErr
	}

	return nil
}

func cloudProviderEventName(regarding corev1.ObjectReference) string {
	hashInput := fmt.Sprintf("%s/%s/%s/%s/%s/%s",
		regarding.Namespace,
		regarding.Kind,
		regarding.Name,
		regarding.UID,
		eventActionCheckClient,
		eventReasonFailedToCheckClient,
	)
	hash := sha256.Sum256([]byte(hashInput))

	return fmt.Sprintf("%s-%s", ProviderName, hex.EncodeToString(hash[:])[:12])
}

func cloudProviderEventObjectReference() corev1.ObjectReference {
	if podName := os.Getenv(podNameEnv); podName != "" {
		return corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       podKind,
			Namespace:  cloudProviderEventNamespace(),
			Name:       podName,
			UID:        types.UID(os.Getenv(podUIDEnv)),
		}
	}

	return corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       componentKind,
		Namespace:  cloudProviderEventNamespace(),
		Name:       ProviderName,
	}
}

func cloudProviderEventNamespace() string {
	if podNamespace := os.Getenv(podNamespaceEnv); podNamespace != "" {
		return podNamespace
	}

	return metav1.NamespaceSystem
}

func cloudProviderEventReportingInstance() string {
	if podName := os.Getenv(podNameEnv); podName != "" {
		return podName
	}

	return cloudControllerManagerClientName
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
	return xok8s.ProviderName
}

// HasClusterID is not implemented.
func (c *cloud) HasClusterID() bool {
	return true
}
