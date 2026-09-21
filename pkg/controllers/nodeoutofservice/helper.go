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
	"time"

	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	cloudnodehelpers "k8s.io/cloud-provider/node/helpers"
)

// OutOfServiceTaintValue is the conventional value used for the
// node.kubernetes.io/out-of-service taint (Non-Graceful Node Shutdown).
const OutOfServiceTaintValue = "nodeshutdown"

// outOfServiceTaint builds the Non-Graceful Node Shutdown taint.
//
// This taint is read by kube-controller-manager only:
//   - the Pod GC controller force-deletes the pods of a NotReady node that
//     carries the taint (pkg/controller/podgc/gc_controller.go, gcTerminating);
//   - the Attach/Detach controller skips the "still mounted" guard and the
//     maxWaitForUnmountDuration (6 min) force-detach timer
//     (pkg/controller/volume/attachdetach/reconciler/reconciler.go,
//     hasOutOfServiceTaint).
//
// See https://kubernetes.io/docs/concepts/cluster-administration/node-shutdown/#non-graceful-node-shutdown
// and KEP-2268 (feature gate NodeOutOfServiceVolumeDetach, GA since 1.28).
func outOfServiceTaint() *v1.Taint {
	return &v1.Taint{
		Key:    v1.TaintNodeOutOfService,
		Value:  OutOfServiceTaintValue,
		Effect: v1.TaintEffectNoExecute,
	}
}

// hasOutOfServiceTaint returns true when the node already carries the taint.
func hasOutOfServiceTaint(node *v1.Node) bool {
	for i := range node.Spec.Taints {
		if node.Spec.Taints[i].Key == v1.TaintNodeOutOfService {
			return true
		}
	}
	return false
}

// isNodeReady reports whether the node Ready condition is True.
func isNodeReady(node *v1.Node) bool {
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == v1.NodeReady {
			return node.Status.Conditions[i].Status == v1.ConditionTrue
		}
	}
	return false
}

// shouldApplyOutOfServiceTaint decides whether the out-of-service taint must be
// added to the node.
//
//	instanceDown   the XO VM is halted/shutdown or no longer exists
//	instanceExists false when the VM was deleted from Xen Orchestra
//	nodeReady      the node Ready condition
//	firstObserved  when the (instanceDown && !nodeReady) pair was first seen
//	grace          how long the pair must hold before tainting a shutdown VM
//
// A VM that no longer exists can never come back, so it is tainted right away.
// A VM that is merely powered off may be rebooting, so we wait for the grace
// period before tainting it (a clean reboot must not trigger a force detach).
// This follows the upstream warning that only a node which is really out of
// service must be tainted.
//
// The caller must have filtered out the nodes that are not managed by this
// cloud provider.
func shouldApplyOutOfServiceTaint(node *v1.Node, instanceDown, instanceExists, nodeReady bool, firstObserved, now time.Time, grace time.Duration) bool {
	if hasOutOfServiceTaint(node) {
		return false
	}
	if !instanceDown || nodeReady {
		return false
	}
	if !instanceExists {
		return true
	}
	return !firstObserved.IsZero() && now.Sub(firstObserved) >= grace
}

// shouldRemoveOutOfServiceTaint decides whether the taint must be removed, so a
// node whose VM came back and whose kubelet reports Ready can be reused.
func shouldRemoveOutOfServiceTaint(node *v1.Node, instanceRunning, nodeReady bool) bool {
	return hasOutOfServiceTaint(node) && instanceRunning && nodeReady
}

// addOutOfServiceTaint patches the node through the API (idempotent).
func addOutOfServiceTaint(c clientset.Interface, node *v1.Node) error {
	return cloudnodehelpers.AddOrUpdateTaintOnNode(c, node.Name, outOfServiceTaint())
}

// removeOutOfServiceTaint removes the taint (no-op when absent).
func removeOutOfServiceTaint(c clientset.Interface, node *v1.Node) error {
	return cloudnodehelpers.RemoveTaintOffNode(c, node.Name, node.DeepCopy(), outOfServiceTaint())
}
