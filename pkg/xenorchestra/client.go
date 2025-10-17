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

// Package xenorchestra implements the multi-cloud provider interface for Xen Orchestra.
package xenorchestra

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid"

	"github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/provider"
	xocfg "github.com/vatesfr/xenorchestra-go-sdk/pkg/config"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/services/library"
	v2 "github.com/vatesfr/xenorchestra-go-sdk/v2"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// XoClient is a Xen Orchestra client.
type XoClient struct {
	config *XoConfig
	Client library.Library
}

// newXOClient creates a new Xen Orchestra client.
func NewXOClient(cfg *XoConfig) (*XoClient, error) {
	// Replace the http/https in the URL with ws/wss
	// to use WebSocket for the connection.
	// This is required for the Xen Orchestra API to work correctly.
	// If the URL is not using http/https, it will be used as is.
	if cfg.URL != "" {
		if cfg.URL[:4] == "http" {
			if cfg.URL[:5] == "https" {
				cfg.URL = "wss" + cfg.URL[5:]
			} else {
				cfg.URL = "ws" + cfg.URL[4:]
			}
		}
	}

	xoConfig := &xocfg.Config{
		Url:                cfg.URL,
		Username:           cfg.Username,
		Password:           cfg.Password,
		Token:              cfg.Token,
		InsecureSkipVerify: cfg.Insecure,
	}

	client, err := v2.New(xoConfig)
	if err != nil {
		klog.Errorf("Failed to create Xen Orchestra client: %v", err)

		return nil, err
	}

	return &XoClient{
		config: cfg,
		Client: client,
	}, nil
}

// CheckClient checks if the Xen Orchestra connection is working.
func (c *XoClient) CheckClient(ctx context.Context) error {
	vms, err := c.Client.VM().GetAll(ctx, 1, "")
	if err != nil {
		return fmt.Errorf("failed to get list of VMs, error: %v", err)
	}

	if len(vms) > 0 {
		klog.V(4).InfoS("Xen Orchestra instance has VMs", "count", len(vms))
	} else {
		klog.InfoS("Xen Orchestra instance has no VMs, or check the account permission")
	}

	return nil
}

// FindVMByNode find a VM by kubernetes node resource in Xen Orchestra.
// It returns the VM, the pool ID (region) and an error if any.
func (c *XoClient) FindVMByNode(ctx context.Context, node *v1.Node) (vm *payloads.VM, poolID uuid.UUID, err error) {
	var vmID uuid.UUID
	if node.Status.NodeInfo.SystemUUID == "" {
		vmID, err = provider.GetVMID(node.Spec.ProviderID)
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("node SystemUUID is empty: %v", err)
		}
	} else {
		vmID, err = uuid.FromString(node.Status.NodeInfo.SystemUUID)
		if err != nil {
			return nil, uuid.Nil, fmt.Errorf("invalid SystemUUID format: %v", err)
		}
	}

	vmClient := c.Client.VM()

	vm, err = vmClient.GetByID(ctx, vmID)
	if err != nil {
		return nil, uuid.Nil, err
	}

	klog.V(4).InfoS("Found VM by node", "vm", vm.NameLabel, "uuid", vm.ID.String())

	return vm, vm.PoolID, nil
}

// FindVMByName find a VM by name in Xen Orchestra (all pools).
func (c *XoClient) FindVMByName(ctx context.Context, name string) (*payloads.VM, uuid.UUID, error) {
	vmClient := c.Client.VM()

	allVms, err := vmClient.GetAll(ctx, 0, "name_label:"+name)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("failed to get list of VMs, error: %v", err)
	}

	for _, vm := range allVms {
		if vm.NameLabel == name {
			klog.V(4).InfoS("Found VM by name", "vm", vm.NameLabel)

			return vm, vm.PoolID, nil
		}
	}

	klog.V(4).InfoS("VM not found by name", "name", name)

	return nil, uuid.Nil, fmt.Errorf("vm '%s' not found", name)
}
