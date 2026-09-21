# Node out-of-service taint (Non-Graceful Node Shutdown)

The `cloud-node-out-of-service` controller applies the
`node.kubernetes.io/out-of-service=nodeshutdown:NoExecute` taint to nodes whose
Xen Orchestra VM is no longer running. This is the Kubernetes
[Non-Graceful Node Shutdown](https://kubernetes.io/docs/concepts/cluster-administration/node-shutdown/#non-graceful-node-shutdown)
procedure, implemented by the CCM because it owns the cloud instance state.

## The problem it solves

When a VM dies (hard shutdown, host failure), its kubelet never unmounts the
volumes. The kube-controller-manager attach/detach controller then refuses to
release the `VolumeAttachment` of the dead node until
`maxWaitForUnmountDuration` (**6 minutes**, hardcoded default) has elapsed. Until
then the replacement pod stays in `ContainerCreating` with a `Multi-Attach
error`, so a simple pod restart takes 8 to 10 minutes.

The 6 minute timer itself is not configurable. The supported way to bypass it is
the out-of-service taint: KCM then detaches the volume immediately.

## Who reads the taint

Only the **kube-controller-manager of the workload cluster** reads it, in three
of its controllers:

* **Pod GC controller** (`pkg/controller/podgc/gc_controller.go`, `gcTerminating`):
  force-deletes the terminating pods of a node when
  `!IsNodeReady(node) && taints.TaintKeyExists(node.Spec.Taints, v1.TaintNodeOutOfService)`.
* **Attach/Detach controller**
  (`pkg/controller/volume/attachdetach/reconciler/reconciler.go`,
  `hasOutOfServiceTaint`): skips the "volume is still mounted" guard and the
  `maxWaitForUnmountDuration` force-detach timer, and detaches right away:
  `"DetachVolume started: node has out-of-service taint, force detaching"`.
* **Taint manager** (node lifecycle controller): the taint has effect
  `NoExecute`, so pods without a matching toleration are evicted, which is what
  makes the old pod *terminating* before the Pod GC force-deletes it.

The whole behaviour is gated by the kube-controller-manager feature gate
`NodeOutOfServiceVolumeDetach`, **GA since Kubernetes 1.28 and locked to true**,
so nothing has to be enabled on the cluster.

See also KEP-2268:
<https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/2268-non-graceful-node-shutdown>.

## What the controller does

Every `--node-out-of-service-sync-period` (default `10s`) it lists the nodes and:

1. skips nodes that are not managed by this cloud provider, or that are not
   initialized yet (`node.cloudprovider.kubernetes.io/uninitialized`);
2. **fast path**: skips Ready nodes that do not carry the taint, so a healthy
   cluster makes no Xen Orchestra API call;
3. queries Xen Orchestra with `InstancesV2.InstanceExists` and
   `InstanceShutdown`;
4. applies the taint when the VM is down **and** the node is `NotReady`:
   * the VM was deleted from Xen Orchestra → immediate, it can never come back;
   * the VM is only powered off → only after
     `--node-out-of-service-grace-period` (default `30s`), because a clean reboot
     must not trigger a force detach;
5. removes the taint once the VM runs again and the node is `Ready`.

Applying the taint to a node that is not actually shut down can corrupt a
filesystem, so the two conditions (VM down *and* node `NotReady`) and the grace
period are required. See the upstream warning in the Node Shutdowns
documentation.

## Enabling the controller

There is no dedicated enable flag: the controller runs when it is listed in
`--controllers` (or when `*` is used). The Helm chart (`enabledControllers`) and
the `docs/deploy` manifests enable it by default.

## Flags

| flag | default | description |
|---|---|---|
| `--node-out-of-service-sync-period` | `10s` | reconciliation period |
| `--node-out-of-service-grace-period` | `30s` | how long a powered-off VM must stay down (with a NotReady node) before tainting |

With the Helm chart these are set with the `nodeOutOfServiceSyncPeriod` and
`nodeOutOfServiceGracePeriod` values (empty by default, which keeps the built-in
defaults).

## RBAC

The controller only needs `get/list/watch/update/patch` on `nodes`, which the
CCM already has.

## Measured impact

On a lab cluster (CAPI + Talos + XCP-ng, `vates-csi` volume on an NFS SR),
cutting the VM that runs a database and measuring until the pod is Ready again:

| | without the taint | with the taint |
|---|---:|---:|
| "old VolumeAttachment released" phase | 337–440 s | **15 s** |
| total cut → pod Ready | 482–567 s | **85 s** |

The remaining delay is the `node-monitor-grace-period` (40 s) needed for the node
to be marked `NotReady`.
