# Install

Xen Orchestra Cloud Controller Manager (CCM) supports controllers:
* cloud-node
* cloud-node-lifecycle
* cloud-node-label-sync

`cloud-node` - detects new node launched in the cluster and registers them in the cluster.
Assigns labels and taints based on Xen Orchestra VM configuration.

`cloud-node-lifecycle` - detects node deletion on Xen Orchestra side and removes them from the cluster.

`cloud-node-label-sync` - syncs labels from Xen Orchestra VM to Kubernetes node.

## Requirements

You need to set `--cloud-provider=external` in the kubelet argument for all nodes in the cluster.
The flag informs the kubelet to offload cloud-specific responsibilities to this external component like Xen Orchestra CCM.

```shell
kubelet --cloud-provider=external
```

Otherwise, kubelet will attempt to manage the node's lifecycle by itself, which can cause issues in environments using an external Cloud Controller Manager (CCM).

## Create a Xen Orchestra token

Official [documentation](https://docs.xcp-ng.org/management/manage-at-scale/xo-api/#authentication)

You can use the Xen Orchestra UI, inside your user space to create a token.


Alternatively, you can use xo-cli to create an authentication token:

```shell
$ xo-cli --createToken xoa.company.lan admin@admin.net
Password: ********
Successfully logged with admin@admin.net
Authentication token created

DiYBFavJwf9GODZqQJs23eAx9eh3KlsRhBi8RcoX0KM
```
> [!IMPORTANT]  
> Only admin users can currently use the API.

## Deploy CCM

Create the Xen Orchestra credentials config file `config.yaml`:

```yaml
# Url the Xen Orchestra API
url: https://xoa.example.com
insecure: false
# Token from the previous step
token: "123ABC"
```

### Method 1: kubectl

Create a secret into the kubernetes, from the `config.yaml` file:

```shell
kubectl -n kube-system create secret generic xenorchestra-cloud-controller-manager --from-file=config.yaml
```

Deploy Xen Orchestra CCM with `cloud-node,cloud-node-lifecycle,cloud-node-label-sync` controllers

```shell
kubectl apply -f https://raw.githubusercontent.com/vatesfr/xenorchestra-cloud-controller-manager/main/docs/deploy/cloud-controller-manager.yml
```

### Method 2: helm chart

Create the config file

```yaml
# xo-ccm.yaml
# -- Xen Orchestra cluster config.
config:
  url: http://xo.example.com
  insecure: false
  token: "ABC..."
# Additional values are available in the chart.
logVerbosityLevel: 5
...
```

Deploy Xen Orchestra CCM

```
helm upgrade -i --namespace=kube-system -f xo-ccm.yml \
    xenorchestra-cloud-controller-manager \
    oci://ghcr.io/vatesfr/charts/xenorchestra-cloud-controller-manager
```

More options you can find [here](../charts/xenorchestra-cloud-controller-manager/README.md#values)

## Troubleshooting

Read the [Kubernetes documentation for Cloud Controller Manager Administration](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).

How `kubelet` works with flag `cloud-provider=external`:

1. kubelet join the cluster and send the `Node` object to the API server.
Node object has values:
    * `node.cloudprovider.kubernetes.io/uninitialized` taint.
    * `nodeInfo` field with system information.
2. CCM detects the new node and sends a request to the Xen Orchestra API to get the VM configuration. Like VMID, hostname, etc.
3. CCM updates the `Node` object with labels, taints and `providerID` field. The `providerID` is immutable and has the format `xenorchestra://$POOLID/$VMID`, it cannot be changed after the first update.
4. CCM removes the `node.cloudprovider.kubernetes.io/uninitialized` taint.

If `kubelet` does not have `cloud-provider=external` flag, kubelet will assume that no external CCM is running and will manage the node lifecycle by itself.
This can cause issues with the Xen Orchestra CCM, essentially the CCM will skip the node and will not update the `Node` object.

If you modify the `kubelet` flags, it's recommended to check all workloads in the cluster.
Please __delete__ the node resource first, and __restart__ the kubelet.

The steps to troubleshoot the Xen Orchestra CCM:
1. Scale down the CCM deployment to 1 replica.
2. Set log level to `--v=5` in the deployment.
3. Check the logs
4. Check kubelet flag `--cloud-provider=external`, delete the node resource and restart the kubelet.
5. Check the logs
6. Wait for 1 minute. If CCM cannot reach the Xen Orchestra API, it will log the error.
7. Check tains, labels, and providerID in the `Node` object.
