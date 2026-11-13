# Terraform state in the datastore

Burrito stores Terraform and OpenTofu state in its datastore for layers that use
the default local backend. No `backend` block is needed in the repository.

For each plan or apply run, Burrito locks the layer state, loads the stored state
into the runner's local backend, runs the operation, and saves the resulting
state back to the datastore. State uses the datastore's configured namespace
encryption settings. A layer's state key is based on its Kubernetes namespace
and name, so each layer has an independent state file.

This applies to operations started by Burrito. Developers running `tofu plan`
directly on a checkout still use OpenTofu's local state unless they configure a
backend themselves. The OpenTofu cloud login and local CLI integration are not
part of this workflow.

State locks are held in the datastore process, so run a single datastore
replica when using this feature. Existing configurations with a custom backend
should keep using that backend; Burrito's state synchronization is intended for
configurations using OpenTofu's default local backend.
