/*
Copyright 2026 Vatesfr.

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
package nodeoutofservice

import (
	"context"
	"errors"
	"time"

	"github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/xenorchestra"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	genericcontrollermanager "k8s.io/controller-manager/app"
	controller "k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"
)

const (
	// ControllerName is the fully qualified controller name.
	ControllerName string = "cloud-node-out-of-service-controller"
	// ControllerAlias is the short name usable with --controllers.
	ControllerAlias string = "cloud-node-out-of-service"
)

// Defaults for the command line flags, bound in main.go.
const (
	DefaultSyncPeriod  = 10 * time.Second
	DefaultGracePeriod = 30 * time.Second
)

// Flags, bound to the command line in main.go.
var (
	// SyncPeriod is how often the controller watches all nodes.
	SyncPeriod = DefaultSyncPeriod
	// GracePeriod is how long a powered-off VM must stay down while its node is
	// NotReady before the out-of-service taint is applied. A missing VM (deleted
	// from Xen Orchestra) is tainted immediately.
	GracePeriod = DefaultGracePeriod
)

// Controller applies the node.kubernetes.io/out-of-service taint to the nodes
// whose Xen Orchestra VM is no longer running, so that kube-controller-manager
// can force-detach their volumes without waiting for the 6 minute
// maxWaitForUnmountDuration timer (Non-Graceful Node Shutdown).
//
// It watches the Xen Orchestra VM state instead of the node conditions, because
// the CCM already owns the cloud instance state and this is the earliest
// reliable signal that the node will never shut down cleanly.
type Controller struct {
	nodeInformer     coreinformers.NodeInformer
	eventBroadcaster record.EventBroadcaster
	recorder         record.EventRecorder
	kubeClient       clientset.Interface

	nodesLister        corelisters.NodeLister
	nodeInformerSynced cache.InformerSynced

	cloud       cloudprovider.Interface
	i           xenorchestra.XOInstances
	gracePeriod time.Duration

	// firstObserved records, per node UID, when the "VM down and node NotReady"
	// condition was first observed. Only touched by the single sync goroutine.
	firstObserved map[string]time.Time
}

// StartControllerWrapper is the app.InitFuncConstructor used by main.go.
func StartControllerWrapper(initContext app.ControllerInitContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) app.InitFunc {
	return func(ctx context.Context, controllerContext genericcontrollermanager.ControllerContext) (controller.Interface, bool, error) {
		return startController(ctx, initContext, controllerContext, completedConfig, cloud)
	}
}

func startController(ctx context.Context, initContext app.ControllerInitContext,
	_ genericcontrollermanager.ControllerContext,
	completedConfig *cloudcontrollerconfig.CompletedConfig,
	cloud cloudprovider.Interface,
) (controller.Interface, bool, error) {
	c, err := NewController(
		ctx,
		completedConfig.SharedInformers.Core().V1().Nodes(),
		completedConfig.ClientBuilder.ClientOrDie(initContext.ClientName),
		cloud,
		GracePeriod,
	)
	if err != nil {
		klog.Warningf("failed to start cloud-node-out-of-service controller: %s", err)
		return nil, false, nil
	}

	klog.InfoS("Starting cloud-node-out-of-service controller",
		"controller", ControllerName, "syncPeriod", SyncPeriod.String(), "gracePeriod", GracePeriod.String())
	go c.Run(ctx)

	return nil, true, nil
}

// NewController builds the controller.
func NewController(
	ctx context.Context,
	nodeInformer coreinformers.NodeInformer,
	kubeClient clientset.Interface,
	cloud cloudprovider.Interface,
	gracePeriod time.Duration,
) (*Controller, error) {
	instances, _ := cloud.InstancesV2()

	eventBroadcaster := record.NewBroadcaster(record.WithContext(ctx))

	return &Controller{
		nodeInformer:       nodeInformer,
		kubeClient:         kubeClient,
		eventBroadcaster:   eventBroadcaster,
		recorder:           eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: ControllerName}),
		cloud:              cloud,
		nodesLister:        nodeInformer.Lister(),
		nodeInformerSynced: nodeInformer.Informer().HasSynced,
		i:                  instances.(xenorchestra.XOInstances),
		gracePeriod:        gracePeriod,
		firstObserved:      map[string]time.Time{},
	}, nil
}

// Name returns the controller name.
func (c *Controller) Name() string {
	return ControllerName
}

// Run starts the controller loop.
func (c *Controller) Run(ctx context.Context) {
	stopCh := ctx.Done()

	defer utilruntime.HandleCrash()

	c.eventBroadcaster.StartStructuredLogging(3)
	c.eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: c.kubeClient.CoreV1().Events("")})
	defer c.eventBroadcaster.Shutdown()

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.nodeInformerSynced); !ok {
		klog.Errorf("failed to wait for caches to sync")
		return
	}

	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		if err := c.SyncNodes(ctx); err != nil {
			klog.ErrorS(err, "failed to sync out-of-service taint")
		}
	}, SyncPeriod)

	<-stopCh
}

// SyncNodes reconciles the out-of-service taint for every managed node.
func (c *Controller) SyncNodes(ctx context.Context) error {
	nodes, err := c.nodesLister.List(labels.Everything())
	if err != nil {
		return err
	}

	now := time.Now()
	for _, item := range nodes {
		node := item.DeepCopy()

		if !isManagedNode(node) {
			continue
		}
		// Node not initialized by cloud-node yet: leave it alone.
		if xenorchestra.GetCloudProviderTaint(node.Spec.Taints) != nil {
			continue
		}

		key := string(node.UID)
		nodeReady := isNodeReady(node)

		// Fast path: a Ready node without the taint needs no Xen Orchestra
		// lookup. On a healthy cluster this controller makes no API call at
		// all; it only queries XO when a node is NotReady, or when it still
		// carries the taint and may need it removed.
		if nodeReady && !hasOutOfServiceTaint(node) {
			delete(c.firstObserved, key)
			continue
		}

		vm, err := c.i.GetInstance(ctx, node)
		instanceMissing := errors.Is(err, cloudprovider.InstanceNotFound)
		if err != nil && !instanceMissing {
			klog.ErrorS(err, "Failed to get the instance", "node", klog.KObj(node))
			continue
		}

		// PowerState is Halted, Paused or Suspended when the VM is not running;
		// a deleted VM has no state at all.
		instanceState := "deleted"
		instanceRunning := false
		if !instanceMissing {
			instanceState = vm.PowerState
			instanceRunning = vm.PowerState == payloads.PowerStateRunning
		}
		instanceDown := !instanceRunning

		if instanceDown && !nodeReady {
			if _, seen := c.firstObserved[key]; !seen {
				c.firstObserved[key] = now
			}
		} else {
			delete(c.firstObserved, key)
		}

		switch {
		case shouldApplyOutOfServiceTaint(node, instanceDown, !instanceMissing, nodeReady, c.firstObserved[key], now, c.gracePeriod):
			klog.InfoS("Applying out-of-service taint: VM is not running and node is not Ready",
				"node", klog.KObj(node), "instanceState", instanceState)
			if err := addOutOfServiceTaint(c.kubeClient, node); err != nil {
				klog.ErrorS(err, "Failed to apply out-of-service taint", "node", klog.KObj(node))
				continue
			}
			c.recorder.Eventf(node, v1.EventTypeWarning, "ApplyingOutOfServiceTaint",
				"VM state is %q and the node is not Ready: applying %s=%s:%s so volumes can detach",
				instanceState, v1.TaintNodeOutOfService, OutOfServiceTaintValue, v1.TaintEffectNoExecute)

		case shouldRemoveOutOfServiceTaint(node, instanceRunning, nodeReady):
			klog.InfoS("Removing out-of-service taint: VM is running again and node is Ready", "node", klog.KObj(node))
			if err := removeOutOfServiceTaint(c.kubeClient, node); err != nil {
				klog.ErrorS(err, "Failed to remove out-of-service taint", "node", klog.KObj(node))
				continue
			}
			c.recorder.Eventf(node, v1.EventTypeNormal, "RemovingOutOfServiceTaint",
				"VM is running again and the node is Ready: removing %s", v1.TaintNodeOutOfService)
		}
	}

	return nil
}
