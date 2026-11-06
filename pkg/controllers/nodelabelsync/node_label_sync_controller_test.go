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
package nodelabelsync

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	cloudprovider "k8s.io/cloud-provider"
)

// fakeInstances implements xenorchestra.XOInstances and records the nodes
// passed to InstanceMetadata.
type fakeInstances struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeInstances) GetInstance(context.Context, *v1.Node) (*payloads.VM, error) {
	return nil, nil
}

func (f *fakeInstances) InstanceExists(context.Context, *v1.Node) (bool, error) {
	return true, nil
}

func (f *fakeInstances) InstanceShutdown(context.Context, *v1.Node) (bool, error) {
	return false, nil
}

func (f *fakeInstances) InstanceMetadata(_ context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, node.Name)

	return &cloudprovider.InstanceMetadata{}, nil
}

func TestUpdateNodeLabelsSkipsUnmanagedNodes(t *testing.T) {
	nodes := []*v1.Node{
		{ObjectMeta: metav1.ObjectMeta{Name: "xo"}, Spec: v1.NodeSpec{ProviderID: "xenorchestra://pool-id/vm-id"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "no-provider-id"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "aws"}, Spec: v1.NodeSpec{ProviderID: "aws:///eu-west-1a/i-123"}},
	}

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	objects := make([]runtime.Object, 0, len(nodes))
	for _, n := range nodes {
		require.NoError(t, indexer.Add(n.DeepCopy()))
		objects = append(objects, n.DeepCopy())
	}

	inst := &fakeInstances{}
	c := &Controller{
		kubeClient:  fake.NewSimpleClientset(objects...),
		nodesLister: corelisters.NewNodeLister(indexer),
		i:           inst,
		recorder:    record.NewFakeRecorder(10),
		workerCount: 1,
	}

	require.NoError(t, c.UpdateNodeLabels(context.Background()))
	assert.Equal(t, []string{"xo"}, inst.calls)
}
