package provider

import (
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"testing"
)

func TestAccJailResource_CreateUpdateDestroy(t *testing.T) {
	backend := newFakeRestshim()
	srv := backend.server(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_jail" "test" {
  id       = "jail-1"
  name     = "web-1"
  hostname = "web-1.local"
  node_id  = "node-a"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apiary_jail.test", "id", "jail-1"),
					resource.TestCheckResourceAttr("apiary_jail.test", "name", "web-1"),
					resource.TestCheckResourceAttr("apiary_jail.test", "hostname", "web-1.local"),
					resource.TestCheckResourceAttr("apiary_jail.test", "node_id", "node-a"),
				),
			},
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_jail" "test" {
  id       = "jail-1"
  name     = "web-1-renamed"
  hostname = "web-1.local"
  node_id  = "node-a"
}
`,
				Check: resource.TestCheckResourceAttr("apiary_jail.test", "name", "web-1-renamed"),
			},
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_jail" "test" {
  id       = "jail-1"
  name     = "web-1-renamed"
  hostname = "web-1.local"
  node_id  = "node-a"
}
`,
				ResourceName:      "apiary_jail.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccJailResource_ReplicaNodeID(t *testing.T) {
	backend := newFakeRestshim()
	srv := backend.server(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_jail" "test" {
  id              = "jail-1"
  node_id         = "node-a"
  replica_node_id = "node-b"
}
`,
				Check: resource.TestCheckResourceAttr("apiary_jail.test", "replica_node_id", "node-b"),
			},
		},
	})
}
