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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudproviderapi "k8s.io/cloud-provider/api"
)

func testNode(opts ...func(*v1.Node)) *v1.Node {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1", UID: types.UID("uid-1")},
		Spec:       v1.NodeSpec{ProviderID: "xenorchestra://pool-id/vm-id"},
		Status: v1.NodeStatus{Conditions: []v1.NodeCondition{
			{Type: v1.NodeReady, Status: v1.ConditionTrue, Reason: "KubeletReady"},
		}},
	}
	for _, o := range opts {
		o(node)
	}
	return node
}

func notReady(n *v1.Node) {
	for i := range n.Status.Conditions {
		if n.Status.Conditions[i].Type == v1.NodeReady {
			n.Status.Conditions[i].Status = v1.ConditionFalse
		}
	}
}

func noCondition(n *v1.Node) {
	n.Status.Conditions = nil
}

func withCloudTaint(n *v1.Node) {
	n.Spec.Taints = append(n.Spec.Taints, v1.Taint{
		Key: cloudproviderapi.TaintExternalCloudProvider, Effect: v1.TaintEffectNoSchedule,
	})
}

func withOutOfServiceTaint(n *v1.Node) {
	n.Spec.Taints = append(n.Spec.Taints, *outOfServiceTaint())
}

func TestOutOfServiceTaint(t *testing.T) {
	taint := outOfServiceTaint()
	assert.Equal(t, v1.TaintNodeOutOfService, taint.Key)
	assert.Equal(t, OutOfServiceTaintValue, taint.Value)
	assert.Equal(t, v1.TaintEffectNoExecute, taint.Effect)
}

func TestIsNodeReady(t *testing.T) {
	assert.True(t, isNodeReady(testNode()))
	assert.False(t, isNodeReady(testNode(notReady)))
	assert.False(t, isNodeReady(testNode(noCondition)))
}

func TestHasOutOfServiceTaint(t *testing.T) {
	assert.False(t, hasOutOfServiceTaint(testNode()))
	assert.True(t, hasOutOfServiceTaint(testNode(withOutOfServiceTaint)))
}

func TestShouldApplyOutOfServiceTaint(t *testing.T) {
	now := time.Now()
	grace := 30 * time.Second
	old := now.Add(-time.Minute)
	fresh := now.Add(-time.Second)

	tests := []struct {
		name     string
		node     *v1.Node
		down     bool
		exists   bool
		ready    bool
		observed time.Time
		expected bool
	}{
		{"already tainted", testNode(withOutOfServiceTaint), true, true, false, old, false},
		{"instance running", testNode(notReady), false, true, false, time.Time{}, false},
		{"instance down but node ready", testNode(), true, true, true, old, false},
		{"shutdown, not ready, never observed", testNode(notReady), true, true, false, time.Time{}, false},
		{"shutdown, not ready, within grace", testNode(notReady), true, true, false, fresh, false},
		{"shutdown, not ready, grace expired", testNode(notReady), true, true, false, old, true},
		{"missing instance, not ready, never observed", testNode(notReady), true, false, false, time.Time{}, true},
		{"missing instance, not ready, within grace", testNode(notReady), true, false, false, fresh, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldApplyOutOfServiceTaint(tt.node, tt.down, tt.exists, tt.ready, tt.observed, now, grace)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestShouldRemoveOutOfServiceTaint(t *testing.T) {
	assert.True(t, shouldRemoveOutOfServiceTaint(testNode(withOutOfServiceTaint), true, true))
	assert.False(t, shouldRemoveOutOfServiceTaint(testNode(withOutOfServiceTaint), true, false))
	assert.False(t, shouldRemoveOutOfServiceTaint(testNode(withOutOfServiceTaint), false, true))
	assert.False(t, shouldRemoveOutOfServiceTaint(testNode(), true, true))
}
