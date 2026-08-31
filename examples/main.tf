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
  desired_state = "running"

  # Optional: HAST-replicate this VM's disk to a second node for data
  # redundancy (not automatic failover - see Apiary's ADR-0026).
  # replica_node_id = "freebsd-apiary"
}

resource "apiary_jail" "app" {
  id       = "app-1"
  name     = "app-1"
  hostname = "app-1.local"
  node_id  = "apiarium"
}
