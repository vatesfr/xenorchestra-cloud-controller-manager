/*
Copyright 2025 Vatesfr

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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	v1 "k8s.io/api/core/v1"
	cloudproviderapi "k8s.io/cloud-provider/api"
)

func TestGetCloudProviderTaint(t *testing.T) {
	nodeTaint := v1.Taint{Key: cloudproviderapi.TaintExternalCloudProvider, Effect: v1.TaintEffectNoSchedule}
	otherTaint := v1.Taint{Key: "example.com/other", Effect: v1.TaintEffectNoExecute}

	assert.Nil(t, GetCloudProviderTaint(nil))
	assert.Nil(t, GetCloudProviderTaint([]v1.Taint{otherTaint}))
	assert.Equal(t, &nodeTaint, GetCloudProviderTaint([]v1.Taint{otherTaint, nodeTaint}))
}

func TestIsNodeManagedByXO(t *testing.T) {
	assert.True(t, IsNodeManagedByXO(&v1.Node{Spec: v1.NodeSpec{ProviderID: "xenorchestra://pool-id/vm-id"}}))
	assert.False(t, IsNodeManagedByXO(&v1.Node{Spec: v1.NodeSpec{ProviderID: "aws:///eu-west-1a/i-123"}}))
	assert.False(t, IsNodeManagedByXO(&v1.Node{}))
}

func TestGetInstanceType(t *testing.T) {
	tests := []struct {
		name     string
		vm       *payloads.VM
		expected string
	}{
		{
			name: "2 vCPU 4 GB",
			vm: &payloads.VM{
				CPUs:   payloads.CPUs{Max: 2},
				Memory: payloads.Memory{Size: 4 * 1024 * 1024 * 1024},
			},
			expected: "2vCPU-4GB",
		},
		{
			name: "4 vCPU 8 GB",
			vm: &payloads.VM{
				CPUs:   payloads.CPUs{Max: 4},
				Memory: payloads.Memory{Size: 8 * 1024 * 1024 * 1024},
			},
			expected: "4vCPU-8GB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getInstanceType(tt.vm)
			assert.Equal(t, tt.expected, result)
		})
	}
}

//nolint:goconst // nolint for readability
func TestSanitizeToLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "clean string", input: "hello-world", expected: "hello-world"},
		{name: "spaces replaced", input: "hello world", expected: "hello-world"},
		{name: "leading dash trimmed", input: "-hello", expected: "hello"},
		{name: "trailing dash trimmed", input: "hello-", expected: "hello"},
		{name: "truncated to 63", input: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", expected: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeToLabel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
