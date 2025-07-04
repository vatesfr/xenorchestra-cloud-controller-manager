/*
Copyright 2025 Vatesfr

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
	"strings"

	"github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/xenorchestra"

	v1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"
	cloudnodeutil "k8s.io/cloud-provider/node/helpers"
	"k8s.io/klog/v2"
)

// updateNodeLabels updates the labels of a single node
func updateNodeLabels(kubeClient clientset.Interface, node *v1.Node, instanceMetadata *cloudprovider.InstanceMetadata) {
	klog.V(5).Infof("NodeLabelSyncController.updateNodeLabels(): sync node %s", node.Name)
	// Do not process nodes that are still tainted
	cloudTaint := getCloudTaint(node.Spec.Taints)
	if cloudTaint != nil {
		klog.V(5).Infof("This node %s is still tainted. Will not process.", node.Name)
		return
	}

	additionalLabels := instanceMetadata.AdditionalLabels
	if len(additionalLabels) == 0 {
		klog.V(5).Infof("Skipping node label update for node %q since cloud provider did not return any", node.Name)
		return
	}

	nodeLabels := node.Labels
	labelsToUpdate := map[string]string{}
	for key, value := range additionalLabels {
		if !strings.Contains(key, xenorchestra.XOLabelNamespace) {
			continue
		}
		if nodeVal, exists := nodeLabels[key]; !exists || nodeVal != value {
			labelsToUpdate[key] = value
			if !exists {
				klog.V(2).Infof("Adding additional node label(s) from XO cloud provider: %s=%s", key, value)
			}
		}
	}

	/**
	 * If label "zone" changed: Node has migrated to a new host (vm.$container field)
	 */
	// Check if the existing node label for zone differs from the new instance metadata Zone value
	existingZone, hasZone := nodeLabels[v1.LabelTopologyZone]
	if hasZone && instanceMetadata.Zone != "" && existingZone != instanceMetadata.Zone {
		klog.V(2).Infof("Node %s zone label changed (VM host has changed): old=%s, new=%s", node.Name, existingZone, instanceMetadata.Zone)
		if _, exists := nodeLabels[xenorchestra.XOLabelTopologyOriginalHostID]; !exists {
			labelsToUpdate[xenorchestra.XOLabelTopologyOriginalHostID] = nodeLabels[v1.LabelTopologyZone]
		}
		labelsToUpdate[v1.LabelTopologyZone] = instanceMetadata.Zone
		labelsToUpdate[v1.LabelFailureDomainBetaZone] = instanceMetadata.Zone
		// TODO: throw an event to k8s to notify the VM host has changed
	}
	/**
	 * If label "region" changed: The node VM has migrated to another pool
	 */
	existingRegion, hasRegion := nodeLabels[v1.LabelTopologyRegion]
	if hasRegion && instanceMetadata.Region != "" && existingRegion != instanceMetadata.Region {
		klog.V(2).Infof("Node %s region label changed (VM pool has changed): old=%s, new=%s", node.Name, existingRegion, instanceMetadata.Region)
		if _, exists := nodeLabels[xenorchestra.XOLabelTopologyOriginalPoolID]; !exists {
			labelsToUpdate[xenorchestra.XOLabelTopologyOriginalPoolID] = nodeLabels[v1.LabelTopologyRegion]
		}
		labelsToUpdate[v1.LabelTopologyRegion] = instanceMetadata.Region
		labelsToUpdate[v1.LabelFailureDomainBetaRegion] = instanceMetadata.Region
		// TODO: throw an event to k8s to notify the VM pool has changed
	}
	// Update VM Type
	existVMType, hasVMType := nodeLabels[v1.LabelInstanceTypeStable]
	if hasVMType && instanceMetadata.InstanceType != "" && existVMType != instanceMetadata.InstanceType {
		klog.V(2).Infof("Node %s VM type label changed (VM type has changed): old=%s, new=%s", node.Name, existVMType, instanceMetadata.InstanceType)
		labelsToUpdate[v1.LabelInstanceTypeStable] = instanceMetadata.InstanceType
		labelsToUpdate[v1.LabelInstanceType] = instanceMetadata.InstanceType
	}

	// Remove obsolete k8s.xenorchestra labels
	// FIXME: do not delete added labels like OriginalZone
	// for key := range nodeLabels {
	// 	if strings.Contains(key, xenorchestra.XoLabelNamespace) {
	// 		if _, exists := additionalLabels[key]; !exists {
	// 			labelsToUpdate[key] = ""
	// 			klog.V(2).Infof("Removing obsolete node label from XO cloud provider: %s", key)
	// 		}
	// 	}
	// }

	if len(labelsToUpdate) == 0 {
		klog.V(5).Infof("Skipping node label update for node %q since there are no changes", node.Name)
		return
	}

	klog.V(4).InfoS("Updating node labels for node %q", "node", node.Name, "labelsToUpdate", labelsToUpdate)

	if !cloudnodeutil.AddOrUpdateLabelsOnNode(kubeClient, labelsToUpdate, node) {
		klog.Error("error updating labels for the node", "node", klog.KRef("", node.Name))
	}
}

func getCloudTaint(taints []v1.Taint) *v1.Taint {
	for _, taint := range taints {
		if taint.Key == cloudproviderapi.TaintExternalCloudProvider {
			return &taint
		}
	}
	return nil
}
