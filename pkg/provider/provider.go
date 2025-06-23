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

// Package provider implements the providerID interface for Xen Orchestra.
package provider

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gofrs/uuid"
	xosdk "github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
)

const (
	// ProviderName is the name of the Xen Orchestra provider.
	ProviderName = "xenorchestra"
)

var providerIDRegexp = regexp.MustCompile(`^` + ProviderName + `://([^/]*)/([^/]+)$`)

// GetProviderID returns the magic providerID for kubernetes node.
func GetProviderID(pool uuid.UUID, vm *xosdk.VM) string {
	poolID := ""
	if !pool.IsNil() {
		poolID = pool.String()
	}

	return fmt.Sprintf("%s://%s/%s", ProviderName, poolID, vm.ID.String())
}

// GetProviderIDFromUUID returns the magic providerID for kubernetes node.
func GetProviderIDFromUUID(uuid uuid.UUID) string {
	return fmt.Sprintf("%s://%s", ProviderName, uuid)
}

// GetVMID returns the VM ID from the providerID.
func GetVMID(providerID string) (uuid.UUID, error) {
	if !strings.HasPrefix(providerID, ProviderName) {
		return uuid.Nil, fmt.Errorf("foreign providerID or empty \"%s\"", providerID)
	}

	vm, _, err := ParseProviderID(providerID)
	if err != nil {
		return uuid.Nil, err
	}

	return vm.ID, nil
}

// ParseProviderID returns the VmRef and region/pool from the providerID.
func ParseProviderID(providerID string) (*xosdk.VM, uuid.UUID, error) {
	if !strings.HasPrefix(providerID, ProviderName) {
		return nil, uuid.Nil, fmt.Errorf("foreign providerID or empty \"%s\"", providerID)
	}

	matches := providerIDRegexp.FindStringSubmatch(providerID)
	if len(matches) != 3 {
		return nil, uuid.Nil, fmt.Errorf("providerID \"%s\" didn't match expected format \"%s://PoolID/InstanceID\"", providerID, ProviderName)
	}

	vmID, err := uuid.FromString(matches[2])
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("InstanceID have to be an UUID, but got \"%s\"", matches[2])
	}

	xoVM := &xosdk.VM{
		ID: vmID,
	}

	poolID := uuid.FromStringOrNil(matches[1])
	if poolID.IsNil() && matches[1] != "" {
		return nil, uuid.Nil, fmt.Errorf("PoolID have to be an UUID, but got \"%s\"", matches[1])
	}

	xoVM.PoolID = poolID

	return xoVM, poolID, nil
}
