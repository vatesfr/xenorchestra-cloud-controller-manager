/*
Copyright 2023 The Kubernetes Authors.

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

package xenorchestra

import (
	"context"
	"fmt"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/golang/mock/gomock"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	mock_library "github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/xenorchestra/mocks"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
	xok8s "github.com/vatesfr/xenorchestra-k8s-common"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"
)

type ccmTestSuite struct {
	suite.Suite
	ctrl *gomock.Controller
	i    *instances
}

func (ts *ccmTestSuite) SetupTest() {
	// Mock VM service
	ts.ctrl = gomock.NewController(ts.T())

	mockVM := mock_library.NewMockVM(ts.ctrl)
	mockVM.EXPECT().GetAll(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*payloads.VM{
		{
			ID:        uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440001")),
			NameLabel: "test1-vm",
			PoolID:    uuid.Must(uuid.FromString("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d")),
			Container: uuid.FromStringOrNil("8af7110d-bfad-407a-a663-9527d10a6583"),
		},
		{
			ID:        uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440002")),
			NameLabel: "test2-vm",
			PoolID:    uuid.Must(uuid.FromString("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d")),
			Container: uuid.FromStringOrNil("8af7110d-bfad-407a-a663-9527d10a6583"),
		},
	}, nil).AnyTimes()

	mockVM.EXPECT().GetByID(gomock.Any(), uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440001"))).Return(
		&payloads.VM{
			ID:            uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440001")),
			NameLabel:     "pool-1-node-1",
			PoolID:        uuid.Must(uuid.FromString("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d")),
			Container:     uuid.FromStringOrNil("8af7110d-bfad-407a-a663-9527d10a6583"),
			CPUs:          payloads.CPUs{Max: 4},
			Memory:        payloads.Memory{Size: 10 * 1024 * 1024 * 1024},
			PowerState:    "Running",
			MainIpAddress: "10.0.0.1",
		}, nil).AnyTimes()

	mockVM.EXPECT().GetByID(gomock.Any(), uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440002"))).Return(
		&payloads.VM{
			ID:            uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440002")),
			NameLabel:     "pool-2-node-1",
			PoolID:        uuid.Must(uuid.FromString("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f2d")),
			Container:     uuid.FromStringOrNil("8af7110d-bfad-407a-a663-9527d10a6586"),
			CPUs:          payloads.CPUs{Max: 2},
			Memory:        payloads.Memory{Size: 4 * 1024 * 1024 * 1024},
			PowerState:    "Halted",
			MainIpAddress: "10.0.0.2",
		}, nil).AnyTimes()

	// Mock VM for testing host retrieval error
	mockVM.EXPECT().GetByID(gomock.Any(), uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440003"))).Return(
		&payloads.VM{
			ID:            uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440003")),
			NameLabel:     "pool-1-node-3",
			PoolID:        uuid.Must(uuid.FromString("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d")),
			Container:     uuid.FromStringOrNil("8af7110d-bfad-407a-a663-9527d10a6584"),
			CPUs:          payloads.CPUs{Max: 8},
			Memory:        payloads.Memory{Size: 16 * 1024 * 1024 * 1024},
			PowerState:    "Running",
			MainIpAddress: "10.0.0.3",
		}, nil).AnyTimes()

	// Mock VM for testing pool retrieval error
	mockVM.EXPECT().GetByID(gomock.Any(), uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440004"))).Return(
		&payloads.VM{
			ID:            uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440004")),
			NameLabel:     "pool-z-node-4",
			PoolID:        uuid.Must(uuid.FromString("ffffffff-ffff-4fff-afff-ffffffffffff")),
			Container:     uuid.FromStringOrNil("8af7110d-bfad-407a-a663-9527d10a6586"),
			CPUs:          payloads.CPUs{Max: 2},
			Memory:        payloads.Memory{Size: 4 * 1024 * 1024 * 1024},
			PowerState:    "Running",
			MainIpAddress: "10.0.0.4",
		}, nil).AnyTimes()

	mockVM.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf(`API error: 404 Not Found - {
	"error": "no such VM 48b8425b-469a-4b4b-860e-635568e5445a"
}`)).AnyTimes()

	// Mock Host service
	mockHost := mock_library.NewMockHost(ts.ctrl)
	mockHost.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil("8af7110d-bfad-407a-a663-9527d10a6583")).Return(
		&payloads.Host{
			ID:        uuid.FromStringOrNil("8af7110d-bfad-407a-a663-9527d10a6583"),
			NameLabel: "test-host-1",
		}, nil).AnyTimes()

	mockHost.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil("8af7110d-bfad-407a-a663-9527d10a6586")).Return(
		&payloads.Host{
			ID:        uuid.FromStringOrNil("8af7110d-bfad-407a-a663-9527d10a6586"),
			NameLabel: "test-host-2",
		}, nil).AnyTimes()

	// Mock Host.Get for a specific UUID that will return an error
	mockHost.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil("8af7110d-bfad-407a-a663-9527d10a6584")).Return(
		nil, fmt.Errorf("API error: 404 Not Found - host not found")).AnyTimes()

	// // Default mock for any other Host.Get call
	// mockHost.EXPECT().Get(gomock.Any(), gomock.Any()).Return(
	// 	&payloads.Host{
	// 		ID:        uuid.UUID{},
	// 		NameLabel: "default-host",
	// 	}, nil).AnyTimes()

	mockPool := mock_library.NewMockPool(ts.ctrl)
	mockPool.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d")).Return(
		&payloads.Pool{
			ID:        uuid.FromStringOrNil("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d"),
			NameLabel: "test-pool-1",
		}, nil).AnyTimes()

	mockPool.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f2d")).Return(
		&payloads.Pool{
			ID:        uuid.FromStringOrNil("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f2d"),
			NameLabel: "test-pool-2",
		}, nil).AnyTimes()

	mockPool.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil("ffffffff-ffff-4fff-afff-ffffffffffff")).Return(
		nil, fmt.Errorf("API error: 404 Not Found - pool not found")).AnyTimes()

	// Mock Library
	mockLib := mock_library.NewMockLibrary(ts.ctrl)
	mockLib.EXPECT().VM().Return(mockVM).AnyTimes()
	mockLib.EXPECT().Host().Return(mockHost).AnyTimes()
	mockLib.EXPECT().Pool().Return(mockPool).AnyTimes()
	// Inject mock into XOClient
	client := &xok8s.XoClient{
		Client: mockLib,
	}
	ts.i = newInstances(client)
}

func (ts *ccmTestSuite) TearDownTest() {
	ts.ctrl.Finish()
}

func TestSuiteCCM(t *testing.T) {
	suite.Run(t, new(ccmTestSuite))
}

// nolint:dupl
func (ts *ccmTestSuite) TestInstanceExists() {
	tests := []struct {
		msg           string
		node          *v1.Node
		expectedError string
		expected      bool
	}{
		{
			msg: "EmptyProviderID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
			},
			expected: true,
		},
		{
			msg: "NodeForeignProviderID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "foreign://provider-id",
				},
			},
			expected: true,
		},
		{
			msg: "NodeWrongPool",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-1-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d/550e8400-e29b-41d4-a716-446655440001",
				},
			},
			expected:      false,
			expectedError: "instances.getInstance() error: vm.PoolID=a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d mismatches nodePoolID=a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d",
		},
		{
			msg: "NodeNotExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-1-node-500",
				},
				Spec: v1.NodeSpec{
					ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d/550e8400-e29b-41d4-a716-446655440005",
				},
			},
			expected: false,
		},
		{
			msg: "NodeExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-1-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d/550e8400-e29b-41d4-a716-446655440001",
				},
			},
			expected: true,
		},
		{
			// The node VM is expected to be found. It can be renamed from XO
			msg: "NodeExistsWithDifferentName",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-1-node-3",
				},
				Spec: v1.NodeSpec{
					ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d/550e8400-e29b-41d4-a716-446655440001",
				},
			},
			expected: true,
		},
	}

	for _, testCase := range tests {
		ts.Run(fmt.Sprint(testCase.msg), func() {
			exists, err := ts.i.InstanceExists(context.Background(), testCase.node)

			if testCase.expectedError != "" {
				ts.Require().Error(err)
				ts.Require().False(exists)
				ts.Require().Contains(err.Error(), testCase.expectedError)
			} else {
				ts.Require().NoError(err)
				ts.Require().Equal(testCase.expected, exists)
			}
		})
	}
}

// nolint:dupl
func (ts *ccmTestSuite) TestInstanceShutdown() {
	tests := []struct {
		msg           string
		node          *v1.Node
		expectedError string
		expected      bool
	}{
		{
			msg: "EptyProviderID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
			},
			expected: false,
		},
		{
			msg: "NodeForeignProviderID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "foreign://provider-id",
				},
			},
			expected: false,
		},
		{
			msg: "NodeNotExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-500",
				},
				Spec: v1.NodeSpec{
					ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d/550e8400-e29b-41d4-a716-446655440005",
				},
			},
			expected:      false,
			expectedError: "vm not found: xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d/550e8400-e29b-41d4-a716-446655440005",
		},
		{
			msg: "NodeExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-1-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d/550e8400-e29b-41d4-a716-446655440001",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-bfad-407a-a663-9527d10a6583",
					},
				},
			},
			expected: false,
		},
		{
			msg: "NodeExistsStopped",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-2",
				},
				Spec: v1.NodeSpec{
					ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f2d/550e8400-e29b-41d4-a716-446655440002",
				},
			},
			expected: true,
		},
	}

	for _, testCase := range tests {
		ts.Run(fmt.Sprint(testCase.msg), func() {
			stopped, err := ts.i.InstanceShutdown(context.Background(), testCase.node)

			if testCase.expectedError != "" {
				ts.Require().Error(err)
				ts.Require().False(stopped)
				ts.Require().Contains(err.Error(), testCase.expectedError)
			} else {
				ts.Require().NoError(err)
				ts.Require().Equal(testCase.expected, stopped)
			}
		})
	}
}

func (ts *ccmTestSuite) TestInstanceMetadata() {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		msg           string
		node          *v1.Node
		expectedError string
		expected      *cloudprovider.InstanceMetadata
	}{
		{
			msg: "NodeEmptyProviderAndSystemUUID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
			},
			expectedError: "instances.InstanceMetadata() - failed to find instance by uuid test-node-1: node SystemUUID is empty: foreign providerID or empty \"\", skipped",
		},
		{
			msg: "NodeForeignProviderID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "foreign://provider-id",
				},
			},
			expected: &cloudprovider.InstanceMetadata{},
		},
		{
			msg: "NodeWrongPool",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-3-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d/550e8400-e29b-41d4-a716-446655440001",
				},
			},
			expected:      &cloudprovider.InstanceMetadata{},
			expectedError: "instances.getInstance() error: vm.PoolID=a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d mismatches nodePoolID=a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d",
		},
		{
			msg: "NodeNotExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-1-node-500",
				},
				Spec: v1.NodeSpec{
					ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d/550e8400-e29b-41d4-a716-446655440005",
				},
			},
			expected:      &cloudprovider.InstanceMetadata{},
			expectedError: cloudprovider.InstanceNotFound.Error(),
		},
		{
			msg: "NodeExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-1-node-1",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "550e8400-e29b-41d4-a716-446655440001",
					},
				},
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d/550e8400-e29b-41d4-a716-446655440001",
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: "1.2.3.4",
					},
					{
						Type:    v1.NodeExternalIP,
						Address: "10.0.0.1",
					},
					{
						Type:    v1.NodeHostName,
						Address: "pool-1-node-1",
					},
				},
				InstanceType: "4vCPU-10GB",
				Region:       "a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d",
				Zone:         "8af7110d-bfad-407a-a663-9527d10a6583",
				AdditionalLabels: map[string]string{
					"topology.k8s.xenorchestra/host_id":         "8af7110d-bfad-407a-a663-9527d10a6583",
					"topology.k8s.xenorchestra/pool_id":         "a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d",
					"vm.k8s.xenorchestra/name_label":            "pool-1-node-1",
					"topology.k8s.xenorchestra/pool_name_label": "test-pool-1",
					"topology.k8s.xenorchestra/host_name_label": "test-host-1",
				},
			},
		},
		{
			msg: "NodeExistsDualstack",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-1-node-1",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4,2001::1",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "550e8400-e29b-41d4-a716-446655440001",
					},
				},
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d/550e8400-e29b-41d4-a716-446655440001",
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: "1.2.3.4",
					},
					{
						Type:    v1.NodeInternalIP,
						Address: "2001::1",
					},
					{
						Type:    v1.NodeExternalIP,
						Address: "10.0.0.1",
					},
					{
						Type:    v1.NodeHostName,
						Address: "pool-1-node-1",
					},
				},
				InstanceType: "4vCPU-10GB",
				Region:       "a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d",
				Zone:         "8af7110d-bfad-407a-a663-9527d10a6583",
				AdditionalLabels: map[string]string{
					"topology.k8s.xenorchestra/host_id":         "8af7110d-bfad-407a-a663-9527d10a6583",
					"topology.k8s.xenorchestra/pool_id":         "a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d",
					"vm.k8s.xenorchestra/name_label":            "pool-1-node-1",
					"topology.k8s.xenorchestra/host_name_label": "test-host-1",
					"topology.k8s.xenorchestra/pool_name_label": "test-pool-1",
				},
			},
		},
		{
			msg: "NodeExistsHostRetrievalError",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-1-node-3",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.5",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d/550e8400-e29b-41d4-a716-446655440003",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "550e8400-e29b-41d4-a716-446655440003",
					},
				},
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d/550e8400-e29b-41d4-a716-446655440003",
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: "1.2.3.5",
					},
					{
						Type:    v1.NodeExternalIP,
						Address: "10.0.0.3",
					},
					{
						Type:    v1.NodeHostName,
						Address: "pool-1-node-3",
					},
				},
				InstanceType: "8vCPU-16GB",
				Region:       "a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d",
				Zone:         "8af7110d-bfad-407a-a663-9527d10a6584",
				AdditionalLabels: map[string]string{
					"topology.k8s.xenorchestra/host_id":         "8af7110d-bfad-407a-a663-9527d10a6584",
					"topology.k8s.xenorchestra/pool_id":         "a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d",
					"vm.k8s.xenorchestra/name_label":            "pool-1-node-3",
					"topology.k8s.xenorchestra/host_name_label": "unknown",
					"topology.k8s.xenorchestra/pool_name_label": "test-pool-1",
				},
			},
		},
		{
			msg: "NodeExistsPoolRetrievalError",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool-unknown-node-1",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.6",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: "xenorchestra://ffffffff-ffff-4fff-afff-ffffffffffff/550e8400-e29b-41d4-a716-446655440004",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "550e8400-e29b-41d4-a716-446655440004",
					},
				},
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: "xenorchestra://ffffffff-ffff-4fff-afff-ffffffffffff/550e8400-e29b-41d4-a716-446655440004",
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: "1.2.3.6",
					},
					{
						Type:    v1.NodeExternalIP,
						Address: "10.0.0.4",
					},
					{
						Type:    v1.NodeHostName,
						Address: "pool-unknown-node-1",
					},
				},
				InstanceType: "2vCPU-4GB",
				Region:       "ffffffff-ffff-4fff-afff-ffffffffffff",
				Zone:         "8af7110d-bfad-407a-a663-9527d10a6586",
				AdditionalLabels: map[string]string{
					"topology.k8s.xenorchestra/host_id":         "8af7110d-bfad-407a-a663-9527d10a6586",
					"topology.k8s.xenorchestra/pool_id":         "ffffffff-ffff-4fff-afff-ffffffffffff",
					"vm.k8s.xenorchestra/name_label":            "pool-z-node-4",
					"topology.k8s.xenorchestra/host_name_label": "test-host-2",
					"topology.k8s.xenorchestra/pool_name_label": "unknown",
				},
			},
		},
	}

	for _, testCase := range tests {
		ts.Run(fmt.Sprint(testCase.msg), func() {
			meta, err := ts.i.InstanceMetadata(context.Background(), testCase.node)

			if testCase.expectedError != "" {
				ts.Require().Error(err)
				ts.Require().Contains(err.Error(), testCase.expectedError)
			} else {
				ts.Require().NoError(err)
				ts.Require().Equal(testCase.expected, meta)
			}
		})
	}
}

func TestSanitizeLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg            string
		input          string
		expectedString string
	}{
		{
			msg:            "Valid alphanumeric string",
			input:          "test123",
			expectedString: "test123",
		},
		{
			msg:            "Valid string with hyphens and dots",
			input:          "test-node.example",
			expectedString: "test-node.example",
		},
		{
			msg:            "Valid string with underscores",
			input:          "test_node_1",
			expectedString: "test_node_1",
		},
		{
			msg:            "String with invalid characters",
			input:          "test@node#1",
			expectedString: "test-node-1",
		},
		{
			msg:            "String with spaces",
			input:          "test node 1",
			expectedString: "test-node-1",
		},
		{
			msg:            "String with leading invalid characters",
			input:          "###test-node",
			expectedString: "test-node",
		},
		{
			msg:            "String with trailing invalid characters",
			input:          "test-node###",
			expectedString: "test-node",
		},
		{
			msg:            "String with leading and trailing invalid characters",
			input:          "###test-node###",
			expectedString: "test-node",
		},
		{
			msg:            "String longer than 63 characters",
			input:          "this-is-a-very-long-string-that-exceeds-the-maximum-length-of-63-characters-for-kubernetes-labels",
			expectedString: "this-is-a-very-long-string-that-exceeds-the-maximum-length-of-6",
		},
		{
			msg:            "Empty string",
			input:          "",
			expectedString: "",
		},
		{
			msg:            "String with only invalid characters",
			input:          "###@@@",
			expectedString: "",
		},
		{
			msg:            "String with mixed valid and invalid characters",
			input:          "test!@#$%^&*()node",
			expectedString: "test----------node",
		},
		{
			msg:            "String with unicode characters",
			input:          "test-ñode-1",
			expectedString: "test-ñode-1",
		},
		{
			msg:            "String starting with digit",
			input:          "1test-node",
			expectedString: "1test-node",
		},
		{
			msg:            "String ending with digit",
			input:          "test-node1",
			expectedString: "test-node1",
		},
	}

	for _, testCase := range tests {
		t.Run(fmt.Sprint(testCase.msg), func(t *testing.T) {
			t.Parallel()

			output := sanitizeToLabel(testCase.input)
			assert.Equal(t, testCase.expectedString, output)
		})
	}
}
