package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccVMDataSource_LooksUpExistingVM confirms apiary_vm's data
// source reads back a VM the fake backend already knows about -
// standing in for "created by hand, or by another Terraform state" -
// rather than one this same config created.
func TestAccVMDataSource_LooksUpExistingVM(t *testing.T) {
	backend := newFakeRestshim()
	backend.vms["vm-1"] = VM{
		ID: "vm-1", Name: "prod-db", VCPUs: 4, MemoryMB: 8192, NodeID: "node-a",
		ISOName: "debian.iso", NetworkID: "net-1", BaseImageName: "debian-cloud.raw",
		IPAddress: "10.60.0.5", MACAddress: "02:00:00:00:00:01",
	}
	srv := backend.server(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv.URL) + `
data "apiary_vm" "test" {
  id = "vm-1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.apiary_vm.test", "name", "prod-db"),
					resource.TestCheckResourceAttr("data.apiary_vm.test", "vcpus", "4"),
					resource.TestCheckResourceAttr("data.apiary_vm.test", "memory_mb", "8192"),
					resource.TestCheckResourceAttr("data.apiary_vm.test", "node_id", "node-a"),
					resource.TestCheckResourceAttr("data.apiary_vm.test", "iso_name", "debian.iso"),
					resource.TestCheckResourceAttr("data.apiary_vm.test", "network_id", "net-1"),
					resource.TestCheckResourceAttr("data.apiary_vm.test", "base_image_name", "debian-cloud.raw"),
					resource.TestCheckResourceAttr("data.apiary_vm.test", "ip_address", "10.60.0.5"),
					resource.TestCheckResourceAttr("data.apiary_vm.test", "mac_address", "02:00:00:00:00:01"),
				),
			},
		},
	})
}

func TestAccVMDataSource_NotFoundIsAnError(t *testing.T) {
	backend := newFakeRestshim()
	srv := backend.server(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv.URL) + `
data "apiary_vm" "test" {
  id = "does-not-exist"
}
`,
				ExpectError: regexp.MustCompile(`VM not found`),
			},
		},
	})
}

func TestAccJailDataSource_LooksUpExistingJail(t *testing.T) {
	backend := newFakeRestshim()
	backend.jails["jail-1"] = Jail{ID: "jail-1", Name: "web", Hostname: "web.local", NodeID: "node-a", ReplicaNodeID: "node-b"}
	srv := backend.server(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv.URL) + `
data "apiary_jail" "test" {
  id = "jail-1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.apiary_jail.test", "name", "web"),
					resource.TestCheckResourceAttr("data.apiary_jail.test", "hostname", "web.local"),
					resource.TestCheckResourceAttr("data.apiary_jail.test", "node_id", "node-a"),
					resource.TestCheckResourceAttr("data.apiary_jail.test", "replica_node_id", "node-b"),
				),
			},
		},
	})
}

func TestAccNetworkDataSource_LooksUpExistingNetwork(t *testing.T) {
	backend := newFakeRestshim()
	backend.networks["net-1"] = Network{ID: "net-1", Name: "prod", VLANID: 100, Subnet: "10.60.0.0/24", BridgeName: "apnet-abcd1234"}
	srv := backend.server(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv.URL) + `
data "apiary_network" "test" {
  id = "net-1"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.apiary_network.test", "name", "prod"),
					resource.TestCheckResourceAttr("data.apiary_network.test", "vlan_id", "100"),
					resource.TestCheckResourceAttr("data.apiary_network.test", "subnet", "10.60.0.0/24"),
					resource.TestCheckResourceAttr("data.apiary_network.test", "bridge_name", "apnet-abcd1234"),
				),
			},
		},
	})
}
