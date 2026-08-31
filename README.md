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
`apiary_jail`, `apiary_network`.

Live-verified against a real Apiary cluster (`apiarium`, via a real
`restshimd` process and an SSH tunnel): `terraform apply` created a
real `apiary_jail` and `apiary_network`, converging to an actual
running `jail(8)` process on its own ZFS dataset within one reconcile
tick; `terraform plan` reported no drift against that real state;
`terraform destroy` tore both down for real, confirmed via `jls`/
`zfs list` back on the cluster itself.

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
