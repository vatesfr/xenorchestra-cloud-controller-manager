# Fast answers to common questions

## Why was my node deleted?

The `cloud-node-lifecycle` controller deletes a `NotReady` node when the CCM
reports that its VM no longer exists. The CCM reports that only when both of
these hold:

1. Xen Orchestra answers `404` with its own error body, whose `data.id` is the
   requested VM UUID;
2. Xen Orchestra still knows the pool named in the node's providerID
   (`xenorchestra://<pool>/<vm>`).

So a node is deleted only when its VM was really deleted in Xen Orchestra. A
pool disconnect, a pool master reboot or a Xen Orchestra restart never deletes
nodes: the CCM logs `Xen Orchestra does not know the pool` and retries. See
[When Xen Orchestra loses a pool](node-out-of-service.md#when-xen-orchestra-loses-a-pool).

## My VM moved to another pool

The node `providerID` (`xenorchestra://<pool>/<vm>`) is set once, when the node
registers, and is immutable. After a VM moves to another pool, the CCM reports
a pool mismatch error for the node and does not update it.

To register the node with its new pool, delete the node object and restart the
kubelet on the VM:

```shell
kubectl delete node <node-name>
# then, on the VM
systemctl restart kubelet
```

## Xen Orchestra was restarted, what happens?

While Xen Orchestra reconnects to its pools, the VMs of a pool that is not back
yet answer `404`. The CCM checks that Xen Orchestra still knows the pool before
believing a `404`, so during the restart it logs
`Xen Orchestra does not know the pool; is the server disconnected?` and skips
those nodes: no node is deleted and no out-of-service taint is applied. Once the
pools are reconnected, everything resumes on the next sync.

If Xen Orchestra sits behind a reverse proxy, make the proxy answer `502`/`503`
while Xen Orchestra is down, never `404` (see [install](install.md)).

## My node VM is paused or suspended

A paused or suspended VM cannot run pods. When its node is `NotReady`, the CCM
applies the `node.kubernetes.io/out-of-service` taint after the grace period
(see [node out-of-service](node-out-of-service.md)). The taint lets Kubernetes
force-detach the node's volumes, so the pods are rescheduled on another node
and can mount their volumes there.

When you remove a node VM in Xen Orchestra, **delete the VM without its disks**.
If the VM is deleted with its disks while a persistent volume VDI is still
attached to it, that VDI is deleted too and its data is lost. Let the taint
detach the volumes first, or detach the persistent volume disks before deleting
the VM.
