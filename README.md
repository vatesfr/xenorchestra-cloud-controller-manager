# Kubernetes cloud controller manager for Xen Orchestra

The Xen Orchestra Cloud Controller Manager (CCM) registers new nodes, keeps them labeled with Xen Orchestra metadata, and cleans them up when their backing VM disappears. It supports multiple pools, so a single Kubernetes cluster can span several Xen Orchestra pools.

The CCM maps Kubernetes topology labels to Xen Orchestra objects:
* `topology.kubernetes.io/region` → Xen Orchestra pool (`clusters[].region`)
* `topology.kubernetes.io/zone` → host name (VM container)

## 🧐 Supported controllers

* cloud-node — registers nodes, sets `providerID`, node addresses, taints, and Xen Orchestra labels during initialization.
* cloud-node-lifecycle — removes Kubernetes nodes when their VM is deleted in Xen Orchestra.
* cloud-node-label-sync — periodically reconciles Xen Orchestra metadata back to Kubernetes nodes after moves or manual changes, keeping both current and original pool/host labels.

## 🧩 Configuration

```yaml
# config.yaml
# URL of the Xen Orchestra API (http or https)
url: https://xoa.example.com
insecure: false

# Authentication (choose one)
token: "123ABC"
# username: admin@admin.net
# password: "s3cret"
```

* Either `token` **or** `username`/`password` is required.
* `url` must include a scheme; set `insecure: true` only when you explicitly want to skip TLS verification.

## 📌 Node labels and providerID

When a node is initialized, the CCM sets `providerID` to `xenorchestra://<pool-id>/<vm-id>` and applies labels derived from Xen Orchestra metadata. Example:

```yaml
apiVersion: v1
kind: Node
metadata:
  labels:
    node.kubernetes.io/instance-type: 2vCPU-1GB
    topology.kubernetes.io/region: 3679fe1a-d058-4055-b800-d30e1bd2af48
    topology.kubernetes.io/zone: 3d6764fe-dc88-42bf-9147-c87d54a73f21
    topology.k8s.xenorchestra/host_id: 3d6764fe-dc88-42bf-9147-c87d54a73f21
    topology.k8s.xenorchestra/host_name_label: hpmc11
    topology.k8s.xenorchestra/pool_id: 3679fe1a-d058-4055-b800-d30e1bd2af48
    topology.k8s.xenorchestra/pool_name_label: devops-tools-moonshot
    topology.k8s.xenorchestra/original_host_id: 3d6764fe-dc88-42bf-9147-c87d54a73f21
    topology.k8s.xenorchestra/original_pool_id: 3679fe1a-d058-4055-b800-d30e1bd2af48
    vm.k8s.xenorchestra/name_label: cgn-microk8s-recipe---Control-Plane
  name: worker-1
spec:
  providerID: xenorchestra://3679fe1a-d058-4055-b800-d30e1bd2af48/8f0d32f8-3ce5-487f-9793-431bab66c115
```

## 🛠️ Install

See [docs/install.md](docs/install.md) for installation options (manifests and Helm chart) and configuration details.

## 🧑🏻‍💻 FAQ

See [docs/faq.md](docs/faq.md) for answers to common questions.

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

