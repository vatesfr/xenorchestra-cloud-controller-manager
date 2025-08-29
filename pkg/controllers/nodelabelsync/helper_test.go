package nodelabelsync

import (
	"testing"

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
	if result != nil {
		t.Errorf("Expected nil for tainted node, but got %v", result)
	}
}
