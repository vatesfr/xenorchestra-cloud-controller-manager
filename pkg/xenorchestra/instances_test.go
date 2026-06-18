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

const (
	pool1Node1        = "pool-1-node-1"
	pool1Node3        = "pool-1-node-3"
	pool1Node500      = "pool-1-node-500"
	pool2Node1        = "pool-2-node-1"
	pool3Node1        = "pool-3-node-1"
	poolZNode4        = "pool-z-node-4"
	poolUnknownNode1  = "pool-unknown-node-1"
	cluster1Node2     = "cluster-1-node-2"
	cluster1Node500   = "cluster-1-node-500"
	sanitizedTestNode = "test-node"
	runningState      = "Running"
	haltedState       = "Halted"
	nodeExternalIP1   = "10.0.0.1"
	nodeExternalIP2   = "10.0.0.2"
	nodeExternalIP3   = "10.0.0.3"
	nodeExternalIP4   = "10.0.0.4"
	publicIP1         = "1.2.3.4"
	publicIP2         = "1.2.3.5"
	publicIP3         = "1.2.3.6"
	publicIPv6        = "2001::1"
	publicDualStack   = publicIP1 + "," + publicIPv6
	testHost1         = "test-host-1"
	testHost2         = "test-host-2"
	testPool1         = "test-pool-1"
	testPool2         = "test-pool-2"
	testNode1         = "test-node-1"
	instanceType1     = "4vCPU-10GB"
	instanceType2     = "2vCPU-4GB"
	instanceType3     = "8vCPU-16GB"

	labelXOHostID   = "topology.k8s.xenorchestra/host_id"
	labelXOPoolID   = "topology.k8s.xenorchestra/pool_id"
	labelXOHostName = "topology.k8s.xenorchestra/host_name_label"
	labelXOPoolName = "topology.k8s.xenorchestra/pool_name_label"
	labelXOVMName   = "vm.k8s.xenorchestra/name_label"

	vmPool1Node1ID  = "550e8400-e29b-41d4-a716-446655440001"
	vmPool2Node1ID  = "550e8400-e29b-41d4-a716-446655440002"
	vmPool1Node3ID  = "550e8400-e29b-41d4-a716-446655440003"
	vmPoolZNode4ID  = "550e8400-e29b-41d4-a716-446655440004"
	vmMissingID     = "550e8400-e29b-41d4-a716-446655440005"
	vmAPINotFoundID = "48b8425b-469a-4b4b-860e-635568e5445a"

	pool1ID       = "a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d"
	pool2ID       = "a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f2d"
	pool3ID       = "a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d"
	poolMissingID = "ffffffff-ffff-4fff-afff-ffffffffffff"
	host1ID       = "8af7110d-bfad-407a-a663-9527d10a6583"
	hostMissingID = "8af7110d-bfad-407a-a663-9527d10a6584"
	host2ID       = "8af7110d-bfad-407a-a663-9527d10a6586"

	nodeForeignProviderID      = "NodeForeignProviderID"
	nodeForeignProviderURI     = "foreign://provider-id"
	xenorchestraProviderScheme = "xenorchestra://"
	providerURIPool1Node1      = xenorchestraProviderScheme + pool1ID + "/" + vmPool1Node1ID
	providerURIPool2Node1      = xenorchestraProviderScheme + pool2ID + "/" + vmPool2Node1ID
	providerURIPool1Node3      = xenorchestraProviderScheme + pool1ID + "/" + vmPool1Node3ID
	providerURIPoolZNode4      = xenorchestraProviderScheme + poolMissingID + "/" + vmPoolZNode4ID
	providerURIMissingVM       = xenorchestraProviderScheme + pool1ID + "/" + vmMissingID
	providerURIWrongPool       = xenorchestraProviderScheme + pool3ID + "/" + vmPool1Node1ID
	nodeExists                 = "NodeExists"
	nodeNotExists              = "NodeNotExists"
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
			ID:        uuid.Must(uuid.FromString(vmPool1Node1ID)),
			NameLabel: "test1-vm",
			PoolID:    uuid.Must(uuid.FromString(pool1ID)),
			Container: uuid.FromStringOrNil(host1ID),
		},
		{
			ID:        uuid.Must(uuid.FromString(vmPool2Node1ID)),
			NameLabel: "test2-vm",
			PoolID:    uuid.Must(uuid.FromString(pool2ID)),
			Container: uuid.FromStringOrNil(host2ID),
		},
	}, nil).AnyTimes()

	mockVM.EXPECT().GetByID(gomock.Any(), uuid.Must(uuid.FromString(vmPool1Node1ID))).Return(
		&payloads.VM{
			ID:            uuid.Must(uuid.FromString(vmPool1Node1ID)),
			NameLabel:     pool1Node1,
			PoolID:        uuid.Must(uuid.FromString(pool1ID)),
			Container:     uuid.FromStringOrNil(host1ID),
			CPUs:          payloads.CPUs{Max: 4},
			Memory:        payloads.Memory{Size: 10 * 1024 * 1024 * 1024},
			PowerState:    runningState,
			MainIpAddress: nodeExternalIP1,
		}, nil).AnyTimes()

	mockVM.EXPECT().GetByID(gomock.Any(), uuid.Must(uuid.FromString(vmPool2Node1ID))).Return(
		&payloads.VM{
			ID:            uuid.Must(uuid.FromString(vmPool2Node1ID)),
			NameLabel:     pool2Node1,
			PoolID:        uuid.Must(uuid.FromString(pool2ID)),
			Container:     uuid.FromStringOrNil(host2ID),
			CPUs:          payloads.CPUs{Max: 2},
			Memory:        payloads.Memory{Size: 4 * 1024 * 1024 * 1024},
			PowerState:    haltedState,
			MainIpAddress: nodeExternalIP2,
		}, nil).AnyTimes()

	// Mock VM for testing host retrieval error
	mockVM.EXPECT().GetByID(gomock.Any(), uuid.Must(uuid.FromString(vmPool1Node3ID))).Return(
		&payloads.VM{
			ID:            uuid.Must(uuid.FromString(vmPool1Node3ID)),
			NameLabel:     pool1Node3,
			PoolID:        uuid.Must(uuid.FromString(pool1ID)),
			Container:     uuid.FromStringOrNil(hostMissingID),
			CPUs:          payloads.CPUs{Max: 8},
			Memory:        payloads.Memory{Size: 16 * 1024 * 1024 * 1024},
			PowerState:    runningState,
			MainIpAddress: nodeExternalIP3,
		}, nil).AnyTimes()

	// Mock VM for testing pool retrieval error
	mockVM.EXPECT().GetByID(gomock.Any(), uuid.Must(uuid.FromString(vmPoolZNode4ID))).Return(
		&payloads.VM{
			ID:            uuid.Must(uuid.FromString(vmPoolZNode4ID)),
			NameLabel:     poolZNode4,
			PoolID:        uuid.Must(uuid.FromString(poolMissingID)),
			Container:     uuid.FromStringOrNil(host2ID),
			CPUs:          payloads.CPUs{Max: 2},
			Memory:        payloads.Memory{Size: 4 * 1024 * 1024 * 1024},
			PowerState:    runningState,
			MainIpAddress: nodeExternalIP4,
		}, nil).AnyTimes()

	mockVM.EXPECT().GetByID(gomock.Any(), gomock.Any()).Return(
		nil,
		fmt.Errorf("API error: 404 Not Found - {\n\t\"error\": \"no such VM %s\"\n}", vmAPINotFoundID),
	).AnyTimes()

	// Mock Host service
	mockHost := mock_library.NewMockHost(ts.ctrl)
	mockHost.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil(host1ID)).Return(
		&payloads.Host{
			ID:        uuid.FromStringOrNil(host1ID),
			NameLabel: testHost1,
		}, nil).AnyTimes()

	mockHost.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil(host2ID)).Return(
		&payloads.Host{
			ID:        uuid.FromStringOrNil(host2ID),
			NameLabel: testHost2,
		}, nil).AnyTimes()

	// Mock Host.Get for a specific UUID that will return an error
	mockHost.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil(hostMissingID)).Return(
		nil, fmt.Errorf("API error: 404 Not Found - host not found")).AnyTimes()

	// // Default mock for any other Host.Get call
	// mockHost.EXPECT().Get(gomock.Any(), gomock.Any()).Return(
	// 	&payloads.Host{
	// 		ID:        uuid.UUID{},
	// 		NameLabel: "default-host",
	// 	}, nil).AnyTimes()

	mockPool := mock_library.NewMockPool(ts.ctrl)
	mockPool.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil(pool1ID)).Return(
		&payloads.Pool{
			ID:        uuid.FromStringOrNil(pool1ID),
			NameLabel: testPool1,
		}, nil).AnyTimes()

	mockPool.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil(pool2ID)).Return(
		&payloads.Pool{
			ID:        uuid.FromStringOrNil(pool2ID),
			NameLabel: testPool2,
		}, nil).AnyTimes()

	mockPool.EXPECT().Get(gomock.Any(), uuid.FromStringOrNil(poolMissingID)).Return(
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
					Name: testNode1,
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
			},
			expected: true,
		},
		{
			msg: nodeForeignProviderID,
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNode1,
				},
				Spec: v1.NodeSpec{
					ProviderID: nodeForeignProviderURI,
				},
			},
			expected: true,
		},
		{
			msg: "NodeWrongPool",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pool1Node1,
				},
				Spec: v1.NodeSpec{
					ProviderID: providerURIWrongPool,
				},
			},
			expected:      false,
			expectedError: "instances.getInstance() error: vm.PoolID=" + pool1ID + " mismatches nodePoolID=" + pool3ID,
		},
		{
			msg: nodeNotExists,
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pool1Node500,
				},
				Spec: v1.NodeSpec{
					ProviderID: providerURIMissingVM,
				},
			},
			expected: false,
		},
		{
			msg: nodeExists,
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pool1Node1,
				},
				Spec: v1.NodeSpec{
					ProviderID: providerURIPool1Node1,
				},
			},
			expected: true,
		},
		{
			// The node VM is expected to be found. It can be renamed from XO
			msg: "NodeExistsWithDifferentName",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pool1Node3,
				},
				Spec: v1.NodeSpec{
					ProviderID: providerURIPool1Node1,
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
					Name: testNode1,
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
			},
			expected: false,
		},
		{
			msg: nodeForeignProviderID,
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNode1,
				},
				Spec: v1.NodeSpec{
					ProviderID: nodeForeignProviderURI,
				},
			},
			expected: false,
		},
		{
			msg: nodeNotExists,
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: cluster1Node500,
				},
				Spec: v1.NodeSpec{
					ProviderID: providerURIMissingVM,
				},
			},
			expected:      false,
			expectedError: "vm not found: " + providerURIMissingVM,
		},
		{
			msg: nodeExists,
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pool1Node1,
				},
				Spec: v1.NodeSpec{
					ProviderID: providerURIPool1Node1,
				},
			},
			expected: false,
		},
		{
			msg: "NodeExistsStopped",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: cluster1Node2,
				},
				Spec: v1.NodeSpec{
					ProviderID: providerURIPool2Node1,
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
					Name: testNode1,
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
			},
			expectedError: "instances.InstanceMetadata() - failed to find instance by uuid " + testNode1 + ": node SystemUUID is empty: foreign providerID or empty \"\", skipped",
		},
		{
			msg: nodeForeignProviderID,
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNode1,
				},
				Spec: v1.NodeSpec{
					ProviderID: nodeForeignProviderURI,
				},
			},
			expected: &cloudprovider.InstanceMetadata{},
		},
		{
			msg: "NodeWrongPool",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pool3Node1,
				},
				Spec: v1.NodeSpec{
					ProviderID: providerURIWrongPool,
				},
			},
			expected:      &cloudprovider.InstanceMetadata{},
			expectedError: "instances.getInstance() error: vm.PoolID=" + pool1ID + " mismatches nodePoolID=" + pool3ID,
		},
		{
			msg: nodeNotExists,
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pool1Node500,
				},
				Spec: v1.NodeSpec{
					ProviderID: providerURIMissingVM,
				},
			},
			expected:      &cloudprovider.InstanceMetadata{},
			expectedError: cloudprovider.InstanceNotFound.Error(),
		},
		{
			msg: nodeExists,
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pool1Node1,
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: publicIP1,
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: vmPool1Node1ID,
					},
				},
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: providerURIPool1Node1,
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: publicIP1,
					},
					{
						Type:    v1.NodeExternalIP,
						Address: nodeExternalIP1,
					},
					{
						Type:    v1.NodeHostName,
						Address: pool1Node1,
					},
				},
				InstanceType: instanceType1,
				Region:       pool1ID,
				Zone:         host1ID,
				AdditionalLabels: map[string]string{
					labelXOHostID:   host1ID,
					labelXOPoolID:   pool1ID,
					labelXOVMName:   pool1Node1,
					labelXOPoolName: testPool1,
					labelXOHostName: testHost1,
				},
			},
		},
		{
			msg: "NodeExistsDualstack",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pool1Node1,
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: publicDualStack,
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: vmPool1Node1ID,
					},
				},
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: providerURIPool1Node1,
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: publicIP1,
					},
					{
						Type:    v1.NodeInternalIP,
						Address: publicIPv6,
					},
					{
						Type:    v1.NodeExternalIP,
						Address: nodeExternalIP1,
					},
					{
						Type:    v1.NodeHostName,
						Address: pool1Node1,
					},
				},
				InstanceType: instanceType1,
				Region:       pool1ID,
				Zone:         host1ID,
				AdditionalLabels: map[string]string{
					labelXOHostID:   host1ID,
					labelXOPoolID:   pool1ID,
					labelXOVMName:   pool1Node1,
					labelXOHostName: testHost1,
					labelXOPoolName: testPool1,
				},
			},
		},
		{
			msg: "NodeExistsHostRetrievalError",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: pool1Node3,
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: publicIP2,
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: providerURIPool1Node3,
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: vmPool1Node3ID,
					},
				},
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: providerURIPool1Node3,
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: publicIP2,
					},
					{
						Type:    v1.NodeExternalIP,
						Address: nodeExternalIP3,
					},
					{
						Type:    v1.NodeHostName,
						Address: pool1Node3,
					},
				},
				InstanceType: instanceType3,
				Region:       pool1ID,
				Zone:         hostMissingID,
				AdditionalLabels: map[string]string{
					labelXOHostID:   hostMissingID,
					labelXOPoolID:   pool1ID,
					labelXOVMName:   pool1Node3,
					labelXOHostName: unknownLabel,
					labelXOPoolName: testPool1,
				},
			},
		},
		{
			msg: "NodeExistsPoolRetrievalError",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: poolUnknownNode1,
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: publicIP3,
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: providerURIPoolZNode4,
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: vmPoolZNode4ID,
					},
				},
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: providerURIPoolZNode4,
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: publicIP3,
					},
					{
						Type:    v1.NodeExternalIP,
						Address: nodeExternalIP4,
					},
					{
						Type:    v1.NodeHostName,
						Address: poolUnknownNode1,
					},
				},
				InstanceType: instanceType2,
				Region:       poolMissingID,
				Zone:         host2ID,
				AdditionalLabels: map[string]string{
					labelXOHostID:   host2ID,
					labelXOPoolID:   poolMissingID,
					labelXOVMName:   poolZNode4,
					labelXOHostName: testHost2,
					labelXOPoolName: unknownLabel,
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
			expectedString: testNode1,
		},
		{
			msg:            "String with spaces",
			input:          "test node 1",
			expectedString: testNode1,
		},
		{
			msg:            "String with leading invalid characters",
			input:          "###test-node",
			expectedString: sanitizedTestNode,
		},
		{
			msg:            "String with trailing invalid characters",
			input:          "test-node###",
			expectedString: sanitizedTestNode,
		},
		{
			msg:            "String with leading and trailing invalid characters",
			input:          "###test-node###",
			expectedString: sanitizedTestNode,
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
