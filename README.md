# Reproducer for fake client SSA behavior

The [`csaupgrade.UpgradeManagedFields`](https://github.com/kubernetes/client-go/blob/df2d882697f9f437d53ae16b8091786250ce0812/util/csaupgrade/upgrade.go#L80) utility is meant to upgrade existing resources from using client-side apply to server-side apply by modifying the managed fields so that "Update" entries of the manager that did CSA are updated to "Apply" using a new manager.

This transformation is done only in memory and is meant to be persisted to the server by the client. Using neither `client.Update(ctx, upgradedObj)` nor `client.Patch(ctx, origObj, client.MergeFrom(upgradedObj))` works correctly and the object ends up with an "Update" record in the managed fields instead of the expected "Apply".

The reproducer requires a running Kubernetes cluster (like [minikube](https://minikube.sigs.k8s.io)) with kubeconfig's current context connected to it. The running cluster
is used to show the difference between the behavior of a real cluster versus the fake client.

It needs an existing namespace where it creates a single `ConfigMap` called `cm` (which is not cleaned up afterwards).

To try this with minikube run something like this:

```shell
minikube start
kubectl create namespace ssa-test
go build
./fake-client-ssa ssa-test
```

NOTE: this is a reproducer for <https://github.com/kubernetes-sigs/controller-runtime/issues/3484>
