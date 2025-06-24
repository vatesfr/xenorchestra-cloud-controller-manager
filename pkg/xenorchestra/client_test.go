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

package xenorchestra_test

import (
	"fmt"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	xenorchestra "github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/xenorchestra"
	mock_library "github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/xenorchestra/mocks"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
)

// func newInstanceEnv() (*xenorchestra.XOConfig, error) {
// 	// TODO: replace with real test environment variables or configuration
// 	cfg, err := xenorchestra.ReadCloudConfig(strings.NewReader(`
// url: http://127.0.0.1:9000
// token: "secret"
// insecure: true
// `))

// 	return &cfg, err
// }

func newMockedVMClient(_ *testing.T, ctrl *gomock.Controller) *xenorchestra.XOClient {
	// Mock VM service
	mockVM := mock_library.NewMockVM(ctrl)
	mockVM.EXPECT().List(gomock.Any()).Return([]*payloads.VM{
		{
			ID:        uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440001")),
			NameLabel: "test1-vm",
			PoolID:    uuid.Must(uuid.FromString("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d")),
		},
		{
			ID:        uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440002")),
			NameLabel: "test2-vm",
			PoolID:    uuid.Must(uuid.FromString("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d")),
		},
	}, nil).AnyTimes()
	// Mock Library
	mockLib := mock_library.NewMockLibrary(ctrl)
	mockLib.EXPECT().VM().Return(mockVM).AnyTimes()

	// Inject mock into XOClient
	return &xenorchestra.XOClient{
		Client: mockLib,
	}
}

// func TestNewClient(t *testing.T) {
// 	cfg, err := newInstanceEnv()
// 	assert.Nil(t, err)
// 	assert.NotNil(t, cfg)

// 	client, err := xenorchestra.NewInstance(&xenorchestra.XOConfig{})
// 	assert.NotNil(t, err)
// 	assert.Nil(t, client)

// 	client, err = xenorchestra.NewInstance(cfg)
// 	assert.Nil(t, err)
// }

func TestCheckInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := newMockedVMClient(t, ctrl)
	err := client.CheckInstance(t.Context())
	assert.Nil(t, err)
}

func TestFindVMByNameNonExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := newMockedVMClient(t, ctrl)

	vm, poolID, err := client.FindVMByName(t.Context(), "non-existing-vm")
	assert.NotNil(t, err)
	assert.Equal(t, uuid.Nil, poolID)
	assert.Nil(t, vm)
	assert.Contains(t, err.Error(), "vm 'non-existing-vm' not found")
}

func TestFindVMByNameExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := newMockedVMClient(t, ctrl)

	tests := []struct {
		msg            string
		vmName         string
		expectedError  error
		expectedVMID   uuid.UUID
		expectedPoolID uuid.UUID
	}{
		{
			msg:           "vm not found",
			vmName:        "non-existing-vm",
			expectedError: fmt.Errorf("vm 'non-existing-vm' not found"),
		},
		{
			msg:            "Test1-VM",
			vmName:         "test1-vm",
			expectedVMID:   uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440001")),
			expectedPoolID: uuid.Must(uuid.FromString("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d")),
		},
		{
			msg:            "Test2-VM",
			vmName:         "test2-vm",
			expectedVMID:   uuid.Must(uuid.FromString("550e8400-e29b-41d4-a716-446655440002")),
			expectedPoolID: uuid.Must(uuid.FromString("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f1d")),
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(fmt.Sprint(testCase.msg), func(t *testing.T) {
			vmr, poolID, err := client.FindVMByName(t.Context(), testCase.vmName)

			if testCase.expectedError == nil {
				assert.Nil(t, err)
				assert.NotNil(t, vmr)
				assert.Equal(t, testCase.expectedVMID, vmr.ID)
				assert.Equal(t, testCase.expectedPoolID, poolID)
			} else {
				assert.NotNil(t, err)
				assert.Equal(t, uuid.Nil, poolID)
				assert.Nil(t, vmr)
				assert.Contains(t, err.Error(), "vm 'non-existing-vm' not found")
			}
		})
	}
}
