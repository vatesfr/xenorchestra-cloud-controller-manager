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

package provider_test

import (
	"fmt"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"

	provider "github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/provider"
	xosdk "github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
)

func TestGetProviderID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg                string
		pool               uuid.UUID
		vmID               uuid.UUID
		expectedProviderID string
	}{
		{
			msg:                "Valid providerID",
			pool:               uuid.Must(uuid.FromString("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d")),
			vmID:               uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426614174000")),
			expectedProviderID: "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d/123e4567-e89b-12d3-a456-426614174000",
		},
		{
			msg:                "No pool",
			pool:               uuid.Nil,
			vmID:               uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426614174001")),
			expectedProviderID: "xenorchestra:///123e4567-e89b-12d3-a456-426614174001",
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(fmt.Sprint(testCase.msg), func(t *testing.T) {
			t.Parallel()

			providerID := provider.GetProviderID(testCase.pool, &xosdk.VM{ID: testCase.vmID})

			assert.Equal(t, testCase.expectedProviderID, providerID)
		})
	}
}

func TestGetVmID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg           string
		providerID    string
		expectedError error
		expectedvmID  uuid.UUID
	}{
		{
			msg:           "Valid VMID",
			providerID:    "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d/123e4567-e89b-12d3-a456-426614174001",
			expectedError: nil,
			expectedvmID:  uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426614174001")),
		},
		{
			msg:           "Valid VMID with empty pool",
			providerID:    "xenorchestra:///123e4567-e89b-12d3-a456-426614174002",
			expectedError: nil,
			expectedvmID:  uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426614174002")),
		},
		{
			msg:           "Invalid providerID format",
			providerID:    "xenorchestra://123e4567-e89b-12d3-a456-426614174002",
			expectedError: fmt.Errorf("providerID \"xenorchestra://123e4567-e89b-12d3-a456-426614174002\" didn't match expected format \"xenorchestra://PoolID/InstanceID\""),
		},
		{
			msg:           "Non xenorchestra providerID",
			providerID:    "cloud:///123e4567-e89b-12d3-a456-426614174003",
			expectedError: fmt.Errorf("foreign providerID or empty \"cloud:///123e4567-e89b-12d3-a456-426614174003\""),
			expectedvmID:  uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426614174003")),
		},
		{
			msg:           "Non xenorchestra providerID",
			providerID:    "cloud://123e4567-e89b-12d3-a456-426614174004",
			expectedError: fmt.Errorf("foreign providerID or empty \"cloud://123e4567-e89b-12d3-a456-426614174004\""),
			expectedvmID:  uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426614174004")),
		},
		{
			msg:           "InValid VMID",
			providerID:    "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d/abc",
			expectedError: fmt.Errorf("InstanceID have to be an UUID, but got \"abc\""),
			expectedvmID:  uuid.Nil,
		},
		{
			msg:           "InValid PoolID",
			providerID:    "xenorchestra://abc/a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d",
			expectedError: fmt.Errorf("PoolID have to be an UUID, but got \"abc\""),
			expectedvmID:  uuid.Nil,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(fmt.Sprint(testCase.msg), func(t *testing.T) {
			t.Parallel()

			VMID, err := provider.GetVMID(testCase.providerID)

			if testCase.expectedError != nil {
				assert.NotNil(t, err)
				assert.Equal(t, testCase.expectedError.Error(), err.Error())
			} else {
				assert.Equal(t, testCase.expectedvmID, VMID)
			}
		})
	}
}

func TestParseProviderID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg            string
		providerID     string
		expectedError  error
		expectedvmID   uuid.UUID
		expectedRegion uuid.UUID
	}{
		{
			msg:            "Valid VMID",
			providerID:     "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d/123e4567-e89b-12d3-a456-426614174001",
			expectedError:  nil,
			expectedvmID:   uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426614174001")),
			expectedRegion: uuid.Must(uuid.FromString("a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d")),
		},
		{
			msg:            "Valid VMID with empty region",
			providerID:     "xenorchestra:///123e4567-e89b-12d3-a456-426614174002",
			expectedError:  nil,
			expectedvmID:   uuid.Must(uuid.FromString("123e4567-e89b-12d3-a456-426614174002")),
			expectedRegion: uuid.Nil,
		},
		{
			msg:           "Invalid providerID format",
			providerID:    "xenorchestra://123e4567-e89b-12d3-a456-426614174003",
			expectedError: fmt.Errorf("providerID \"xenorchestra://123e4567-e89b-12d3-a456-426614174003\" didn't match expected format \"xenorchestra://PoolID/InstanceID\""),
		},
		{
			msg:           "Non xenorchestra providerID",
			providerID:    "cloud:///123e4567-e89b-12d3-a456-426614174004",
			expectedError: fmt.Errorf("foreign providerID or empty \"cloud:///123e4567-e89b-12d3-a456-426614174004\""),
		},
		{
			msg:           "Non xenorchestra providerID",
			providerID:    "cloud://123e4567-e89b-12d3-a456-426614174005",
			expectedError: fmt.Errorf("foreign providerID or empty \"cloud://123e4567-e89b-12d3-a456-426614174005\""),
		},
		{
			msg:           "InValid VMID",
			providerID:    "xenorchestra://a3c8f86b-9c2f-4c3d-8a7b-2d44e6f77f3d/abc",
			expectedError: fmt.Errorf("InstanceID have to be an UUID, but got \"abc\""),
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(fmt.Sprint(testCase.msg), func(t *testing.T) {
			t.Parallel()

			vmr, region, err := provider.ParseProviderID(testCase.providerID)

			if testCase.expectedError != nil {
				assert.NotNil(t, err)
				assert.Equal(t, err.Error(), testCase.expectedError.Error())
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, vmr)
				assert.Equal(t, testCase.expectedvmID, vmr.ID)
				assert.Equal(t, testCase.expectedRegion, region)
			}
		})
	}
}
