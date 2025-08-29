/*
Copyright 2025 Vatesfr.

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
package nodelabelsync

import (
	"context"
	"time"

	"github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/xenorchestra"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion/scheme"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	genericcontrollermanager "k8s.io/controller-manager/app"
	controller "k8s.io/controller-manager/controller"
	"k8s.io/klog/v2"
)

const (
	ControllerName  string = "cloud-node-label-sync-controller"
	ControllerAlias string = "cloud-node-label-sync"
)

// Controller periodically syncs node labels from Xen Orchestra
// based on the logic in InstanceMetadata.
type Controller struct {
	nodeInformer     coreinformers.NodeInformer
	eventBroadcaster record.EventBroadcaster
	recorder         record.EventRecorder
	kubeClient       clientset.Interface

	nodesLister        corelisters.NodeLister
	nodeInformerSynced cache.InformerSynced

	nodeStatusUpdateFrequency time.Duration
	workerCount               int32
	cloud                     cloudprovider.Interface
	i                         xenorchestra.XOInstances
}

func StartNodeLabelSyncControllerWrapper(initContext app.ControllerInitContext, completedConfig *cloudcontrollerconfig.CompletedConfig, cloud cloudprovider.Interface) app.InitFunc {
	return func(ctx context.Context, controllerContext genericcontrollermanager.ControllerContext) (controller.Interface, bool, error) {
		return startNodeLabelSyncController(ctx, initContext, controllerContext, completedConfig, cloud)
	}
}

func startNodeLabelSyncController(ctx context.Context, initContext app.ControllerInitContext,
	controlexContext genericcontrollermanager.ControllerContext,
	completedConfig *cloudcontrollerconfig.CompletedConfig,
	cloud cloudprovider.Interface,
) (controller.Interface, bool, error) {
	// Start the CloudNodeController
	nodeController, err := NewNodeLabelSyncController(
		ctx,
		completedConfig.SharedInformers.Core().V1().Nodes(),
		// cloud node controller uses existing cluster role from node-controller
		completedConfig.ClientBuilder.ClientOrDie(initContext.ClientName),
		cloud,
		completedConfig.ComponentConfig.NodeStatusUpdateFrequency.Duration,
		completedConfig.ComponentConfig.NodeController.ConcurrentNodeSyncs,
	)
	if err != nil {
		klog.Warningf("failed to start cloud node controller: %s", err)
		return nil, false, nil
	}

	klog.InfoS("Starting cloud-node-label-sync controller", "controller", ControllerName)
	go nodeController.Run(ctx, completedConfig.ComponentConfig.KubeCloudShared.RouteReconciliationPeriod.Duration)

	return nil, true, nil
	// return controller, true, nil
}

func NewNodeLabelSyncController(
	ctx context.Context,
	nodeInformer coreinformers.NodeInformer,
	kubeClient clientset.Interface,
	cloud cloudprovider.Interface,
	nodeStatusUpdateFrequency time.Duration,
	workerCount int32,
) (*Controller, error) {
	instances, _ := cloud.InstancesV2()

	eventBroadcaster := record.NewBroadcaster(record.WithContext(ctx))

	return &Controller{
		nodeInformer:              nodeInformer,
		kubeClient:                kubeClient,
		eventBroadcaster:          eventBroadcaster,
		recorder:                  eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: ControllerName}),
		cloud:                     cloud,
		nodesLister:               nodeInformer.Lister(),
		nodeInformerSynced:        nodeInformer.Informer().HasSynced,
		i:                         instances.(xenorchestra.XOInstances),
		nodeStatusUpdateFrequency: nodeStatusUpdateFrequency,
		workerCount:               workerCount,
	}, nil
}

func (c *Controller) Run(ctx context.Context, interval time.Duration) {
	stopCh := ctx.Done()

	defer utilruntime.HandleCrash()

	// Start event broadcasting process
	c.eventBroadcaster.StartStructuredLogging(3)
	c.eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: c.kubeClient.CoreV1().Events("")})
	defer c.eventBroadcaster.Shutdown()

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.nodeInformerSynced); !ok {
		klog.Errorf("failed to wait for caches to sync")
		return
	}

	// Run the controller in a loop, periodically syncing node labels
	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		if err := c.UpdateNodeLabels(ctx); err != nil {
			klog.Errorf("failed to update node status: %v", err)
		}
	}, c.nodeStatusUpdateFrequency)

	<-stopCh
}

func (c *Controller) UpdateNodeLabels(ctx context.Context) error {
	klog.V(5).Info("NodeLabelSyncController.UpdateNodeLabels(): Syncing all nodes")
	start := time.Now()
	nodes, err := c.nodesLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Error monitoring node status: %v", err)
		return err
	}
	defer func() {
		klog.V(2).Infof("Update %d nodes status took %v.", len(nodes), time.Since(start))
	}()

	updateNodeFunc := func(piece int) {
		node := nodes[piece].DeepCopy()
		// Do not process nodes that are still tainted, those will be processed by the cloud-node-controller
		cloudTaint := getCloudTaint(node.Spec.Taints)
		if cloudTaint != nil {
			klog.V(5).Infof("This node %s is still tainted. Will not process.", node.Name)
			return
		}

		instanceMetadata, err := c.i.InstanceMetadata(ctx, node)
		if err != nil {
			klog.Errorf("Error getting instance metadata for node label sync: %v", err)
			return
		}
		updateNodeLabels(c.kubeClient, c.recorder, node, instanceMetadata)
	}

	workqueue.ParallelizeUntil(ctx, int(c.workerCount), len(nodes), updateNodeFunc)
	return nil
}

func (c *Controller) Name() string {
	return ControllerName
}
