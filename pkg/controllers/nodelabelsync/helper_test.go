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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/xenorchestra"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudprovider "k8s.io/cloud-provider"
)

// Create instance metadata with additional labels
var instanceMetadataTest = &cloudprovider.InstanceMetadata{
	AdditionalLabels: map[string]string{
		xenorchestra.XOLabelNamespace + "/test-label": "test-value",
	},
	Zone:         "zone-1",
	Region:       "region-1",
	InstanceType: "vm-type-1",
}

func TestGetNodeLabelUpdate_ReturnsNilForTaintedNode(t *testing.T) {
	// Create a node with cloud taint
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{
				{
					Key:    "node.cloudprovider.kubernetes.io/uninitialized",
					Value:  "true",
					Effect: v1.TaintEffectNoSchedule,
				},
			},
		},
	}

	// Call the function
	result := getNodeLabelUpdate(node, instanceMetadataTest)

	// Assert that the function returns nil for tainted nodes
	assert.Nil(t, result, "Expected nil for tainted node")
}

func TestGetNodeLabelUpdate_AddsOrUpdatesXONamespaceLabels(t *testing.T) {
	// Labels to test
	expectedXOLabel1 := xenorchestra.XOLabelNamespace + "/custom-label"
	expectedXOLabel2 := xenorchestra.XOLabelNamespace + "/another-label"
	existingXOLabel1 := xenorchestra.XOLabelNamespace + "/existing-label"
	existingXOLabel2 := xenorchestra.XOLabelNamespace + "/another-existing-label"

	// Create a node without cloud taint and without existing XO labels
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				existingXOLabel1: "existing-value",
				existingXOLabel2: "another-existing-value",
			},
		},
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{}, // No taints
		},
	}

	// Create instance metadata with XO namespace labels
	instanceMetadata := &cloudprovider.InstanceMetadata{
		AdditionalLabels: map[string]string{
			expectedXOLabel1: "custom-value",
			expectedXOLabel2: "another-value",
			existingXOLabel1: "new-value",
			existingXOLabel2: "another-existing-value",
			"non-xo-label":   "should-be-ignored",
		},
	}

	// Call the function
	result := getNodeLabelUpdate(node, instanceMetadata)

	// Assert that XO namespace labels are included in the result and updated
	assert.Equal(t, "custom-value", result[expectedXOLabel1], "Expected XO label %s to be 'custom-value'", expectedXOLabel1)
	assert.Equal(t, "another-value", result[expectedXOLabel2], "Expected XO label %s to be 'another-value'", expectedXOLabel2)
	assert.Equal(t, "new-value", result[existingXOLabel1], "Expected XO label %s to be 'new-value'", existingXOLabel1)

	// Assert that XO labels are not included if not need to update
	assert.NotContains(t, result, existingXOLabel2, "Expected XO label %s to not be included", existingXOLabel2)

	// Assert that non-XO labels are not included
	assert.NotContains(t, result, "non-xo-label", "Non-XO label should not be included in the result")

	// Assert that the result contains the expected number of XO labels
	xoLabelCount := 0
	for key := range result {
		if strings.Contains(key, xenorchestra.XOLabelNamespace) {
			xoLabelCount++
		}
	}

	assert.Equal(t, 3, xoLabelCount, "Expected 3 XO namespace labels")
}

func TestGetNodeLabelUpdate_ZoneChangedAddsOriginalHostAndUpdatesLabels(t *testing.T) {
	// Node has an existing zone that differs from metadata.Zone
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				v1.LabelTopologyZone: "zone-1",
			},
		},
		Spec: v1.NodeSpec{Taints: []v1.Taint{}},
	}

	meta := &cloudprovider.InstanceMetadata{Zone: "zone-2"}

	result := getNodeLabelUpdate(node, meta)

	assert.Equal(t, "zone-2", result[v1.LabelTopologyZone], "zone label should be updated")
	assert.Equal(t, "zone-2", result[v1.LabelFailureDomainBetaZone], "beta zone label should be updated")
	assert.Equal(t, "zone-1", result[xenorchestra.XOLabelTopologyOriginalHostID], "original host id should be set to previous zone")
}

func TestGetNodeLabelUpdate_ZoneChangedDoesNotOverrideOriginalHostIfExists(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-2",
			Labels: map[string]string{
				v1.LabelTopologyZone:                       "zone-1",
				xenorchestra.XOLabelTopologyOriginalHostID: "already-present",
			},
		},
		Spec: v1.NodeSpec{Taints: []v1.Taint{}},
	}

	meta := &cloudprovider.InstanceMetadata{Zone: "zone-3"}

	result := getNodeLabelUpdate(node, meta)

	assert.Equal(t, "zone-3", result[v1.LabelTopologyZone])
	assert.Equal(t, "zone-3", result[v1.LabelFailureDomainBetaZone])
	assert.NotContains(t, result, xenorchestra.XOLabelTopologyOriginalHostID, "should not override existing original host id label")
}

func TestGetNodeLabelUpdate_RegionChangedAddsOriginalPoolAndUpdatesLabels(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-3",
			Labels: map[string]string{
				v1.LabelTopologyRegion: "region-1",
			},
		},
		Spec: v1.NodeSpec{Taints: []v1.Taint{}},
	}

	meta := &cloudprovider.InstanceMetadata{Region: "region-2"}

	result := getNodeLabelUpdate(node, meta)

	assert.Equal(t, "region-2", result[v1.LabelTopologyRegion], "region label should be updated")
	assert.Equal(t, "region-2", result[v1.LabelFailureDomainBetaRegion], "beta region label should be updated")
	assert.Equal(t, "region-1", result[xenorchestra.XOLabelTopologyOriginalPoolID], "original pool id should be set to previous region")
}

func TestGetNodeLabelUpdate_RegionChangedDoesNotOverrideOriginalPoolIfExists(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-4",
			Labels: map[string]string{
				v1.LabelTopologyRegion:                     "region-1",
				xenorchestra.XOLabelTopologyOriginalPoolID: "already-present",
			},
		},
		Spec: v1.NodeSpec{Taints: []v1.Taint{}},
	}

	meta := &cloudprovider.InstanceMetadata{Region: "region-3"}

	result := getNodeLabelUpdate(node, meta)

	assert.Equal(t, "region-3", result[v1.LabelTopologyRegion])
	assert.Equal(t, "region-3", result[v1.LabelFailureDomainBetaRegion])
	assert.NotContains(t, result, xenorchestra.XOLabelTopologyOriginalPoolID, "should not override existing original pool id label")
}

func TestGetNodeLabelUpdate_InstanceTypeChangedUpdatesBothLabels(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-5",
			Labels: map[string]string{
				v1.LabelInstanceTypeStable: "vm-type-1",
			},
		},
		Spec: v1.NodeSpec{Taints: []v1.Taint{}},
	}

	meta := &cloudprovider.InstanceMetadata{InstanceType: "vm-type-2"}

	result := getNodeLabelUpdate(node, meta)

	assert.Equal(t, "vm-type-2", result[v1.LabelInstanceTypeStable], "stable instance type label should be updated")
	assert.Equal(t, "vm-type-2", result[v1.LabelInstanceType], "beta instance type label should be updated")
}

func TestGetNodeLabelUpdate_NoChangesReturnsEmpty(t *testing.T) {
	// Node has labels that already match metadata
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-6",
			Labels: map[string]string{
				v1.LabelTopologyZone:                          instanceMetadataTest.Zone,
				v1.LabelTopologyRegion:                        instanceMetadataTest.Region,
				v1.LabelInstanceTypeStable:                    instanceMetadataTest.InstanceType,
				xenorchestra.XOLabelNamespace + "/test-label": instanceMetadataTest.AdditionalLabels[xenorchestra.XOLabelNamespace+"/test-label"],
			},
		},
		Spec: v1.NodeSpec{Taints: []v1.Taint{}},
	}

	result := getNodeLabelUpdate(node, instanceMetadataTest)

	assert.Equal(t, 0, len(result), "expected no label updates when nothing changed")
}
