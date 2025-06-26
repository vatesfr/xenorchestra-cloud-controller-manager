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
	"strings"
	"unicode"

	"github.com/gofrs/uuid"

	provider "github.com/vatesfr/xenorchestra-cloud-controller-manager/pkg/provider"
	"github.com/vatesfr/xenorchestra-go-sdk/pkg/payloads"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"
	"k8s.io/klog/v2"
)

type instances struct {
	c *XOClient
}

func newInstances(client *XOClient) *instances {
	return &instances{
		c: client,
	}
}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	klog.V(4).InfoS("instances.InstanceExists() called", "node", klog.KRef("", node.Name))

	if node.Spec.ProviderID == "" {
		klog.V(4).InfoS("instances.InstanceExists() empty providerID, omitting unmanaged node", "node", klog.KObj(node))

		return true, nil
	}

	if !strings.HasPrefix(node.Spec.ProviderID, provider.ProviderName) {
		klog.V(4).InfoS("instances.InstanceExists() omitting unmanaged node", "node", klog.KObj(node), "providerID", node.Spec.ProviderID)

		return true, nil
	}

	if _, err := i.getInstance(ctx, node); err != nil {
		if err == cloudprovider.InstanceNotFound {
			klog.V(4).InfoS("instances.InstanceExists() instance not found", "node", klog.KObj(node), "providerID", node.Spec.ProviderID)

			return false, nil // Return nil, it's not an error: it's expected when the VM has been deleted
		}

		return false, err
	}

	return true, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	klog.V(4).InfoS("instances.InstanceShutdown() called", "node", klog.KRef("", node.Name))

	if node.Spec.ProviderID == "" {
		klog.V(4).InfoS("instances.InstanceShutdown() empty providerID, omitting unmanaged node", "node", klog.KObj(node))

		return false, nil
	}

	if !strings.HasPrefix(node.Spec.ProviderID, provider.ProviderName) {
		klog.V(4).InfoS("instances.InstanceShutdown() omitting unmanaged node", "node", klog.KObj(node), "providerID", node.Spec.ProviderID)

		return false, nil
	}

	vmr, err := i.getInstance(ctx, node)
	if err != nil {
		if err == cloudprovider.InstanceNotFound {
			klog.InfoS("instances.InstanceShutdown() instance not found, is it deleted?", "providerID", node.Spec.ProviderID)

			return false, fmt.Errorf("vm not found: %s", node.Spec.ProviderID) // Vm not found, probably deleted
		}

		klog.ErrorS(err, "instances.InstanceShutdown() failed to parse providerID", "providerID", node.Spec.ProviderID)

		return false, nil
	}

	if vmr.PowerState != payloads.PowerStateRunning {
		return true, nil
	}

	return false, nil
}

// InstanceMetadata returns the instance's metadata. The values returned in InstanceMetadata are
// translated into specific fields in the Node object on registration.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	klog.V(4).InfoS("instances.InstanceMetadata() called", "node", klog.KRef("", node.Name))

	var (
		vmRef  *payloads.VM
		region uuid.UUID // The XCP-ng poolID
		err    error
	)

	providerID := node.Spec.ProviderID
	if providerID == "" {
		klog.V(4).InfoS("instances.InstanceMetadata() empty providerID, trying find node", "node", klog.KObj(node), "uuid", node.Status.NodeInfo.SystemUUID)

		vmRef, region, err = i.c.FindVMByNode(ctx, node)
		if err != nil {
			return nil, fmt.Errorf("instances.InstanceMetadata() - failed to find instance by uuid %s: %v, skipped", node.Name, err)
		}

		providerID = provider.GetProviderID(region, vmRef)
	} else if !strings.HasPrefix(node.Spec.ProviderID, provider.ProviderName) {
		klog.V(4).InfoS("instances.InstanceMetadata() omitting unmanaged node", "node", klog.KObj(node), "providerID", node.Spec.ProviderID)

		return &cloudprovider.InstanceMetadata{}, nil
	}

	if vmRef == nil {
		vmRef, err = i.getInstance(ctx, node)
		if err != nil {
			return nil, err
		}
	}

	addresses := []v1.NodeAddress{}

	if providedIP, ok := node.Annotations[cloudproviderapi.AnnotationAlphaProvidedIPAddr]; ok {
		for _, ip := range strings.Split(providedIP, ",") {
			addresses = append(addresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: ip})
		}
	}

	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: node.Name})

	instanceType := i.getInstanceType(ctx, vmRef)

	return &cloudprovider.InstanceMetadata{
		AdditionalLabels: map[string]string{
			"vm.k8s.xenorchestra/name_label":    sanitizeToLabel(vmRef.NameLabel),
			"topology.k8s.xenorchestra/pool_id": sanitizeToLabel(vmRef.PoolID.String()),
			"topology.k8s.xenorchestra/host_id": sanitizeToLabel(vmRef.Container),
			// TODO: Add pool nameLabel and host nameLabel: requires XO SDK additional features
		},
		ProviderID:    providerID,
		NodeAddresses: addresses,
		InstanceType:  instanceType,
		Zone:          vmRef.Container, // TODO: Use uuid for host ID
		Region:        vmRef.PoolID.String(),
	}, nil
}

// getInstance returns the VM reference, and error for the given node.
func (i *instances) getInstance(ctx context.Context, node *v1.Node) (*payloads.VM, error) {
	klog.V(4).InfoS("instances.getInstance() called", "node", klog.KRef("", node.Name))

	nodeRef, poolID, err := provider.ParseProviderID(node.Spec.ProviderID)
	if err != nil {
		klog.Errorf("Cannot parse Node UUID %s (%s)", nodeRef.ID, node.Name)

		return nil, fmt.Errorf("instances.getInstance() error: %v", err)
	}

	vm, err := i.c.Client.VM().GetByID(ctx, nodeRef.ID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "API error: 404 Not Found") {
			return nil, cloudprovider.InstanceNotFound
		}

		return nil, fmt.Errorf("instances.getInstance() error: %v", err)
	}

	// Exclude case `vm.NameLabel != node.Name ||`, this should only require a refresh, not an 'node not found' error
	if vm.ID != nodeRef.ID || vm.PoolID != poolID {
		klog.Errorf("instances.getInstance() vm.name(%s) != node.name(%s) with uuid=%s", vm.NameLabel, node.Name, nodeRef.ID)

		return nil, fmt.Errorf("instances.getInstance() error: vm.PoolID=%s mismatches nodePoolID=%s", vm.PoolID, poolID)
	}

	klog.V(5).Infof("instances.getInstance() vm %+v", vm)

	return vm, nil
}

// getInstanceType returns the instance type for the given VM.
// It returns the instance type name if it matches the expected format, otherwise it returns a formatted string
// returns: the instance type name or a formatted string with vCPU and memory, and an error if any
func (i *instances) getInstanceType(_ context.Context, vm *payloads.VM) string {
	memory := vm.Memory.Size / (1024 * 1024 * 1024)

	return fmt.Sprintf("%.0fVCPU-%.0fGB",
		float64(vm.CPUs.Max),
		float64(memory))
}

// sanitizeToLabel replaces characters in a string so that it matches the regex:
// (([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?
// and truncates to 63 chars.
func sanitizeToLabel(s string) string {
	// Replace all invalid chars with '-'
	var b strings.Builder

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}

	out := b.String()

	// Remove leading and trailing non-alphanumeric
	out = strings.TrimFunc(out, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	const maxLen = 63
	if len(out) > maxLen {
		out = out[:maxLen]
	}

	return out
}
