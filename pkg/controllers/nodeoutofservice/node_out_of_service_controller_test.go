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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	cloudprovider "k8s.io/cloud-provider"
)

// fakeInstances implements xenorchestra.XOInstances for tests.
type fakeInstances struct {
	exists   bool
	shutdown bool
	err      error
}

func (f *fakeInstances) GetInstance(context.Context, *v1.Node) (*payloads.VM, error) {
	return nil, nil
}

func (f *fakeInstances) InstanceExists(context.Context, *v1.Node) (bool, error) {
	return f.exists, f.err
}

func (f *fakeInstances) InstanceShutdown(context.Context, *v1.Node) (bool, error) {
	return f.shutdown, f.err
}

func (f *fakeInstances) InstanceMetadata(context.Context, *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	return &cloudprovider.InstanceMetadata{}, nil
}

// unused context import guard
var _ = context.Background

func newTestController(t *testing.T, node *v1.Node, inst *fakeInstances, grace time.Duration) (*Controller, *fake.Clientset) {
	t.Helper()
	client := fake.NewSimpleClientset(node.DeepCopy())

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(node.DeepCopy()))

	return &Controller{
		kubeClient:    client,
		nodesLister:   corelisters.NewNodeLister(indexer),
		i:             inst,
		recorder:      record.NewFakeRecorder(10),
		gracePeriod:   grace,
		firstObserved: map[string]time.Time{},
	}, client
}

func getNode(t *testing.T, client *fake.Clientset, name string) *v1.Node {
	t.Helper()
	node, err := client.CoreV1().Nodes().Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	return node
}

func TestSyncNodesAppliesTaintOnShutdownNode(t *testing.T) {
	ctx := context.Background()
	node := testNode(notReady)
	c, client := newTestController(t, node, &fakeInstances{exists: true, shutdown: true}, 0)

	require.NoError(t, c.SyncNodes(ctx))
	assert.True(t, hasOutOfServiceTaint(getNode(t, client, node.Name)))
}

func TestSyncNodesAppliesTaintWhenInstanceIsMissing(t *testing.T) {
	ctx := context.Background()
	node := testNode(notReady)
	// A missing VM can never come back: no grace period is required.
	c, client := newTestController(t, node, &fakeInstances{exists: false}, time.Hour)

	require.NoError(t, c.SyncNodes(ctx))
	assert.True(t, hasOutOfServiceTaint(getNode(t, client, node.Name)))
}

func TestSyncNodesDoesNotTaintRunningNode(t *testing.T) {
	ctx := context.Background()
	node := testNode(notReady)
	c, client := newTestController(t, node, &fakeInstances{exists: true, shutdown: false}, 0)

	require.NoError(t, c.SyncNodes(ctx))
	assert.False(t, hasOutOfServiceTaint(getNode(t, client, node.Name)))
}

func TestSyncNodesDoesNotTaintReadyNode(t *testing.T) {
	ctx := context.Background()
	node := testNode() // Ready
	c, client := newTestController(t, node, &fakeInstances{exists: true, shutdown: true}, 0)

	require.NoError(t, c.SyncNodes(ctx))
	assert.False(t, hasOutOfServiceTaint(getNode(t, client, node.Name)))
}

func TestSyncNodesRespectsGracePeriod(t *testing.T) {
	ctx := context.Background()
	node := testNode(notReady)
	c, client := newTestController(t, node, &fakeInstances{exists: true, shutdown: true}, time.Minute)

	// First pass only records when the condition was first seen.
	require.NoError(t, c.SyncNodes(ctx))
	assert.False(t, hasOutOfServiceTaint(getNode(t, client, node.Name)))

	// The condition has now persisted longer than the grace period.
	c.firstObserved[string(node.UID)] = time.Now().Add(-2 * time.Minute)
	require.NoError(t, c.SyncNodes(ctx))
	assert.True(t, hasOutOfServiceTaint(getNode(t, client, node.Name)))
}

func TestSyncNodesRemovesTaintWhenBackToNormal(t *testing.T) {
	ctx := context.Background()
	node := testNode(withOutOfServiceTaint) // Ready
	c, client := newTestController(t, node, &fakeInstances{exists: true, shutdown: false}, 0)

	require.NoError(t, c.SyncNodes(ctx))
	assert.False(t, hasOutOfServiceTaint(getNode(t, client, node.Name)))
}

func TestSyncNodesSkipsUninitializedNode(t *testing.T) {
	ctx := context.Background()
	node := testNode(notReady, withCloudTaint)
	c, client := newTestController(t, node, &fakeInstances{exists: true, shutdown: true}, 0)

	require.NoError(t, c.SyncNodes(ctx))
	assert.False(t, hasOutOfServiceTaint(getNode(t, client, node.Name)))
}
