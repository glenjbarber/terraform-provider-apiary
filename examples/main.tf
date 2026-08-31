terraform {
  required_providers {
    apiary = {
      source = "registry.terraform.io/glenjbarber/apiary"
    }
  }
}

provider "apiary" {
  # Both can also come from APIARY_ENDPOINT / APIARY_API_KEY env vars.
  endpoint = "http://localhost:8080"
  # api_key = "..."
}

resource "apiary_network" "prod" {
  id      = "net-prod"
  name    = "prod"
  vlan_id = 100
  subnet  = "10.60.0.0/24"
}

resource "apiary_vm" "web" {
  id            = "web-1"
  name          = "web-1"
  vcpus         = 2
  memory_mb     = 2048
  node_id       = "apiarium"
  network_id    = apiary_network.prod.id
  desired_state = "running"

  # Optional: attach an installer/seed image already uploaded via
  # Apiary's UploadISO RPC (see ADR-0017), or seed this VM's disk from
  # a pre-uploaded raw base image instead of creating it blank (see
  # ADR-0031) - required in practice for an unattended Linux VM.
  # iso_name        = "debian.iso"
  # base_image_name = "debian-cloud.raw"

  # Optional: HAST-replicate this VM's disk to a second node for data
  # redundancy (not automatic failover - see Apiary's ADR-0026).
  # replica_node_id = "freebsd-apiary"
}

# ip_address/mac_address are read-only, assigned by Apiary's raft FSM
# once network_id is set - available after apply:
# output "web_ip" { value = apiary_vm.web.ip_address }

resource "apiary_jail" "app" {
  id       = "app-1"
  name     = "app-1"
  hostname = "app-1.local"
  node_id  = "apiarium"
}

# Look up a VM/jail/network this configuration doesn't manage (created
# by hand, another tool, or a separate Terraform state):
# data "apiary_vm" "existing" {
#   id = "some-other-vm"
# }
