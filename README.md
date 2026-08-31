# terraform-provider-apiary

Terraform provider for [Apiary](https://github.com/glenjbarber/apiary), a
FreeBSD-native virtualization platform (bhyve VMs, jails, ZFS/HAST storage).

Talks to Apiary's `restshimd` REST API, authenticating with a bearer
API key (see Apiary's ADR-0023/ADR-0024). It is a thin translation
layer over that REST API - it has no logic of its own about VM/jail/
network state, scheduling, or reconciliation; all of that lives in
Apiary itself.

## Status

Under active development. Resources implemented: `apiary_vm`,
`apiary_jail`, `apiary_network`. Data sources: `apiary_vm`,
`apiary_jail`, `apiary_network` (lookup by id - for referencing a
resource this Terraform state doesn't manage, created by hand, another
tool, or a separate state, distinct from importing it into this
resource's own lifecycle via `terraform import`).

Live-verified against a real Apiary cluster (`apiarium`, via a real
`restshimd` process and an SSH tunnel): `terraform apply` created a
real `apiary_jail` and `apiary_network`, converging to an actual
running `jail(8)` process on its own ZFS dataset within one reconcile
tick; `terraform plan` reported no drift against that real state;
`terraform destroy` tore both down for real, confirmed via `jls`/
`zfs list` back on the cluster itself.

`apiary_vm` gained `iso_name`/`network_id`/`base_image_name` (settable)
and `ip_address`/`mac_address` (computed) once Apiary's own restshim
API gained them alongside ADR-0031 (VM base disk images) - these were
previously entirely absent from this provider, so a VM's network
attachment or installer/base image could only be set through Apiary's
own web UI or gRPC API, never through Terraform.

## Requirements

- Go 1.24+ (to build the provider)
- Terraform 1.0+
- A running Apiary cluster with `restshimd` reachable over HTTP

## Building

```sh
go build .
```

## Using a local build during development

Terraform's [dev overrides](https://developer.terraform.io/terraform/cli/config/config-file#development-overrides-for-provider-developers)
let you point `terraform` at a locally built binary instead of a
registry release. Add to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/glenjbarber/apiary" = "/absolute/path/to/terraform-provider-apiary"
  }
  direct {}
}
```

Then `go build .` in this repo and run `terraform plan`/`apply` in a
directory with an `apiary` provider block - no `terraform init` needed
while a dev override is active.

## Configuration

```hcl
provider "apiary" {
  endpoint = "http://localhost:8080" # or APIARY_ENDPOINT env var
  api_key  = "..."                   # or APIARY_API_KEY env var; omit if Apiary has no keys configured
}
```

See [`examples/main.tf`](examples/main.tf) for a complete example
covering all three resources.

## Resources

### `apiary_vm`

| Attribute         | Type   | Notes                                                                 |
|-------------------|--------|------------------------------------------------------------------------|
| `id`              | string | Required. Caller-chosen, matching Apiary's own no-autogeneration model. |
| `name`            | string | Optional.                                                              |
| `vcpus`           | number | Optional. Apiary defaults to 1 if unset.                               |
| `memory_mb`       | number | Optional. Apiary defaults to 512 if unset.                             |
| `node_id`         | string | Optional. Apiary has no automatic scheduling - this must name a real cluster node. |
| `desired_state`   | string | Optional. `"running"` or `"stopped"`.                                  |
| `replica_node_id` | string | Optional. HAST-replicates this VM's disk to a second node (data redundancy, not failover - see Apiary's ADR-0026). |
| `iso_name`        | string | Optional. Names an image already uploaded via Apiary's `UploadISO` RPC, attached as a CD-ROM device (see ADR-0017). |
| `network_id`      | string | Optional. An `apiary_network` this VM's NIC belongs to (see ADR-0022). |
| `base_image_name` | string | Optional. Seeds this VM's disk from a pre-uploaded raw image on first creation instead of a blank disk - required in practice for an unattended Linux VM (see ADR-0031). |
| `ip_address`      | string | Computed. Assigned by Apiary's raft FSM when `network_id` is set.       |
| `mac_address`     | string | Computed. Assigned by Apiary's raft FSM when `network_id` is set.       |

### `apiary_jail`

| Attribute         | Type   | Notes                                                    |
|-------------------|--------|-----------------------------------------------------------|
| `id`              | string | Required.                                                  |
| `name`            | string | Optional.                                                  |
| `hostname`        | string | Optional. Passed to `jail(8)` as `host.hostname`.          |
| `node_id`         | string | Optional.                                                  |
| `replica_node_id` | string | Optional. See Apiary's ADR-0027.                           |
| `desired_state`   | string | Optional. `"running"` or `"stopped"`.                      |

### `apiary_network`

Every attribute requires replacement on change - Apiary's
`ManagerService` has no update-in-place RPC for networks.

| Attribute       | Type   | Notes                                                     |
|-----------------|--------|-------------------------------------------------------------|
| `id`            | string | Required.                                                    |
| `name`          | string | Optional.                                                    |
| `vlan_id`       | number | Optional. 0/unset means untagged.                            |
| `subnet`        | string | Required. CIDR, e.g. `"10.60.0.0/24"`.                       |
| `bridge_name`   | string | Optional override for the bridge(4) interface name.          |
| `bridge_status` | string | Computed. `"up"`/`"down"`/`"unknown"`, as last reported.      |

All three resources support `terraform import <type>.<name> <id>`.

## Data sources

`apiary_vm`, `apiary_jail`, and `apiary_network` each also exist as a
data source with the same name and attributes (minus `id`'s
"required" distinction - it's the lookup key, everything else is
computed). Use these to reference a resource this Terraform
configuration doesn't manage:

```hcl
data "apiary_vm" "existing" {
  id = "some-other-vm"
}

output "existing_vm_ip" {
  value = data.apiary_vm.existing.ip_address
}
```

## Testing

```sh
go test ./...                 # unit tests (the REST client, no Terraform CLI needed)
TF_ACC=1 go test ./...        # + acceptance tests: real terraform apply/import/destroy
                               # cycles against a fake in-memory restshimd stand-in
```

Acceptance tests need the `terraform` binary on `PATH` but never touch
a real Apiary cluster - each test spins up its own `httptest.Server`
implementing just enough of `restshimd`'s REST surface to exercise the
provider's full lifecycle.
