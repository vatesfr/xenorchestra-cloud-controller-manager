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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/xenorchestra"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	cloudprovider "k8s.io/cloud-provider"
)

var testNode = &v1.Node{
	ObjectMeta: metav1.ObjectMeta{
		Name: "node-nochange",
		Labels: map[string]string{
			v1.LabelTopologyZone:                          "host-1",
			v1.LabelTopologyRegion:                        "pool-1",
			v1.LabelInstanceTypeStable:                    "2vCPU-2GB",
			xenorchestra.XOLabelNamespace + "/test-label": "test-value",
		},
	},
	Spec: v1.NodeSpec{Taints: []v1.Taint{}},
}

func drainEvents(rec *record.FakeRecorder, max int, wait time.Duration) []string {
	events := []string{}
	timeout := time.After(wait)
	for {
		select {
		case e := <-rec.Events:
			events = append(events, e)
			if len(events) >= max {
				return events
			}
		case <-timeout:
			return events
		}
	}
}

func TestUpdateNodeLabels_NoChanges(t *testing.T) {
	ctx := context.TODO()
	client := k8sfake.NewClientset()
	recorder := record.NewFakeRecorder(10)

	_, err := client.CoreV1().Nodes().Create(ctx, testNode, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to seed fake node: %v", err)
	}

	meta := &cloudprovider.InstanceMetadata{
		Zone:         "host-1",
		Region:       "pool-1",
		InstanceType: "2vCPU-2GB",
		AdditionalLabels: map[string]string{
			xenorchestra.XOLabelNamespace + "/test-label": "test-value",
		},
	}

	changed := updateNodeLabels(client, recorder, testNode, meta)
	assert.False(t, changed, "expected no changes")

	// Node labels should remain unchanged
	got, err := client.CoreV1().Nodes().Get(ctx, testNode.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get node: %v", err)
	}
	assert.Equal(t, "host-1", got.Labels[v1.LabelTopologyZone])
	assert.Equal(t, "pool-1", got.Labels[v1.LabelTopologyRegion])
	assert.Equal(t, "2vCPU-2GB", got.Labels[v1.LabelInstanceTypeStable])
	assert.Equal(t, "test-value", got.Labels[xenorchestra.XOLabelNamespace+"/test-label"])

	// No events should be emitted
	evs := drainEvents(recorder, 1, 150*time.Millisecond)
	assert.Len(t, evs, 0, "expected no events")
}

func TestUpdateNodeLabels_ZoneChanged(t *testing.T) {
	ctx := context.TODO()
	client := k8sfake.NewClientset()
	recorder := record.NewFakeRecorder(10)

	_, err := client.CoreV1().Nodes().Create(ctx, testNode, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to seed fake node: %v", err)
	}

	meta := &cloudprovider.InstanceMetadata{Zone: "host-2"}

	changed := updateNodeLabels(client, recorder, testNode, meta)
	assert.True(t, changed, "expected changes due to zone update")

	got, err := client.CoreV1().Nodes().Get(ctx, testNode.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get node: %v", err)
	}
	assert.Equal(t, "host-2", got.Labels[v1.LabelTopologyZone])
	assert.Equal(t, "host-2", got.Labels[v1.LabelFailureDomainBetaZone])
	assert.Equal(t, "host-1", got.Labels[xenorchestra.XOLabelTopologyOriginalHostID])

	evs := drainEvents(recorder, 1, 500*time.Millisecond)
	if assert.Equal(t, len(evs), 1, "expected at least one event") {
		// Fake recorder events are like: "TYPE REASON MESSAGE"
		assert.Contains(t, evs[0], "NodeZoneChanged")
		assert.Contains(t, evs[0], "Warning")
		assert.Contains(t, evs[0], "old=host-1")
		assert.Contains(t, evs[0], "new=host-2")
	}
}

func TestUpdateNodeLabels_RegionChanged(t *testing.T) {
	ctx := context.TODO()
	client := k8sfake.NewClientset()
	recorder := record.NewFakeRecorder(10)

	_, err := client.CoreV1().Nodes().Create(ctx, testNode, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to seed fake node: %v", err)
	}

	meta := &cloudprovider.InstanceMetadata{Region: "pool-2"}

	changed := updateNodeLabels(client, recorder, testNode, meta)
	assert.True(t, changed, "expected changes due to region update")

	got, err := client.CoreV1().Nodes().Get(ctx, testNode.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get node: %v", err)
	}
	assert.Equal(t, "pool-2", got.Labels[v1.LabelTopologyRegion])
	assert.Equal(t, "pool-2", got.Labels[v1.LabelFailureDomainBetaRegion])
	assert.Equal(t, "pool-1", got.Labels[xenorchestra.XOLabelTopologyOriginalPoolID])

	evs := drainEvents(recorder, 1, 500*time.Millisecond)
	if assert.Equal(t, len(evs), 1, "expected at least one event") {
		assert.Contains(t, evs[0], "NodeRegionChanged")
		assert.Contains(t, evs[0], "Warning")
		assert.Contains(t, evs[0], "old=pool-1")
		assert.Contains(t, evs[0], "new=pool-2")
	}
}

func TestUpdateNodeLabels_InstanceTypeChanged(t *testing.T) {
	ctx := context.TODO()
	client := k8sfake.NewClientset()
	recorder := record.NewFakeRecorder(10)

	_, err := client.CoreV1().Nodes().Create(ctx, testNode, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to seed fake node: %v", err)
	}

	meta := &cloudprovider.InstanceMetadata{InstanceType: "2vCPU-4GB"}

	changed := updateNodeLabels(client, recorder, testNode, meta)
	assert.True(t, changed, "expected changes due to instance type update")

	got, err := client.CoreV1().Nodes().Get(ctx, testNode.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get node: %v", err)
	}
	assert.Equal(t, "2vCPU-4GB", got.Labels[v1.LabelInstanceTypeStable])
	assert.Equal(t, "2vCPU-4GB", got.Labels[v1.LabelInstanceType])

	evs := drainEvents(recorder, 1, 500*time.Millisecond)
	if assert.Equal(t, len(evs), 1, "expected at least one event") {
		assert.Contains(t, evs[0], "NodeInstanceTypeHasChanged")
		assert.Contains(t, evs[0], "Normal")
		assert.Contains(t, evs[0], "old=2vCPU-2GB")
		assert.Contains(t, evs[0], "new=2vCPU-4GB")
	}
}

func TestUpdateNodeLabels_APIFailure(t *testing.T) {
	ctx := context.TODO()
	client := k8sfake.NewClientset()
	recorder := record.NewFakeRecorder(10)

	// Make Node update/patch fail
	failReactor := func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("simulated API failure")
	}
	client.PrependReactor("patch", "nodes", failReactor)
	client.PrependReactor("update", "nodes", failReactor)

	_, err := client.CoreV1().Nodes().Create(ctx, testNode, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to seed fake node: %v", err)
	}

	meta := &cloudprovider.InstanceMetadata{Zone: "host-2"}

	changed := updateNodeLabels(client, recorder, testNode, meta)
	assert.False(t, changed, "expected failure to return false")

	got, err := client.CoreV1().Nodes().Get(ctx, testNode.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get node: %v", err)
	}
	// Labels should not have been updated due to API error
	assert.Equal(t, "host-1", got.Labels[v1.LabelTopologyZone])
	assert.Empty(t, got.Labels[v1.LabelFailureDomainBetaZone])
	assert.Empty(t, got.Labels[xenorchestra.XOLabelTopologyOriginalHostID])

	// No events should be emitted when the API update fails
	evs := drainEvents(recorder, 2, 200*time.Millisecond)
	// Ensure none of the expected reasons are present
	if len(evs) > 0 {
		joined := strings.Join(evs, "\n")
		assert.NotContains(t, joined, "NodeZoneChanged")
		assert.NotContains(t, joined, "NodeRegionChanged")
		assert.NotContains(t, joined, "NodeInstanceTypeHasChanged")
	}
}
