# Kubernetes cloud controller manager for Xen Orchestra

The CCM does a few things: it initialises new nodes, applies common labels to them. 

It supports multiple pools, meaning you can have one kubernetes cluster across multiple Xenorchestra pools.

The basic definitions:
* kubernetes label `topology.kubernetes.io/region` is a Xenorchestra pool `clusters[].region`
* kubernetes label `topology.kubernetes.io/zone` is an hypervisor host machine name

This makes it possible to use pods affinity/anti-affinity.

## 🧐 Features

Support controllers:

* cloud-node
  * Updates node resource.
  * Assigns labels and taints based on Xen Orchestra VM configuration.
* cloud-node-lifecycle
  * Cleans up node resource when Xen Orchestra VM is deleted.
*cloud-node-label-sync
  * Syncs labels from Xen Orchestra VM to Kubernetes node.

## Example

```yaml
# cloud provider config
# Url the Xen Orchestra API
url: https://xoa.example.com
insecure: false
# Token for the Xen Orchestra API
token: "123ABC"
```

Node spec result:

```yaml
apiVersion: v1
kind: Node
metadata:
  labels:
    # Type generated base on CPU and RAM
    node.kubernetes.io/instance-type: 2vCPU-1GB
    # Xen Orchestra Pool ID of the node VM Host
    topology.kubernetes.io/region: 3679fe1a-d058-4055-b800-d30e1bd2af48
    # Xen Orchestra ID of the node VM Host
    topology.kubernetes.io/zone: 3d6764fe-dc88-42bf-9147-c87d54a73f21
    # Additional labels based on Xen Orchestra data (beta)
    topology.k8s.xenorchestra/host_id: 3d6764fe-dc88-42bf-9147-c87d54a73f21
    topology.k8s.xenorchestra/pool_id: 3679fe1a-d058-4055-b800-d30e1bd2af48
    vm.k8s.xenorchestra/name_label: cgn-microk8s-recipe---Control-Plane
    ...
  name: worker-1
spec:
  ...
  # providerID - magic string:
  #   xeorchestra://{Pool ID}/{VM ID}
  providerID: xeorchestra://3679fe1a-d058-4055-b800-d30e1bd2af48/8f0d32f8-3ce5-487f-9793-431bab66c115
```

## 🛠️ Install

See [Install](docs/install.md) for installation instructions.

## 🧑🏻‍💻 FAQ

See [FAQ](docs/faq.md) for answers to common questions.

## 🍰 Contributing    

Contributions are what make the open source community such an amazing place to be learn, inspire, and create. Any contributions you make are **greatly appreciated**.

## ➤ License

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Thanks to sergelogvinov for the [initial implementation](https://github.com/sergelogvinov/proxmox-cloud-controller-manager).

