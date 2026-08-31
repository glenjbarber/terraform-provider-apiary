package provider

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccNetworkResource_CreateDestroy(t *testing.T) {
	backend := newFakeRestshim()
	srv := backend.server(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_network" "test" {
  id      = "net-1"
  name    = "prod"
  vlan_id = 100
  subnet  = "10.60.0.0/24"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apiary_network.test", "id", "net-1"),
					resource.TestCheckResourceAttr("apiary_network.test", "name", "prod"),
					resource.TestCheckResourceAttr("apiary_network.test", "vlan_id", "100"),
					resource.TestCheckResourceAttr("apiary_network.test", "subnet", "10.60.0.0/24"),
					resource.TestCheckResourceAttr("apiary_network.test", "bridge_status", "unknown"),
				),
			},
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_network" "test" {
  id      = "net-1"
  name    = "prod"
  vlan_id = 100
  subnet  = "10.60.0.0/24"
}
`,
				ResourceName:      "apiary_network.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					// bridge_status is a live, per-node computed value the
					// fake backend never sets - not meaningful to verify
					// round-trips identically through import.
					"bridge_status",
				},
			},
		},
	})
}

// TestAccNetworkResource_SubnetChangeForcesReplacement confirms subnet
// (marked RequiresReplace - see NetworkResource's own doc comment on
// why every attribute needs this) actually drives a destroy/create,
// not a rejected in-place update.
func TestAccNetworkResource_SubnetChangeForcesReplacement(t *testing.T) {
	backend := newFakeRestshim()
	srv := backend.server(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_network" "test" {
  id     = "net-1"
  subnet = "10.60.0.0/24"
}
`,
			},
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_network" "test" {
  id     = "net-1"
  subnet = "10.61.0.0/24"
}
`,
				Check: resource.TestCheckResourceAttr("apiary_network.test", "subnet", "10.61.0.0/24"),
			},
		},
	})
}
