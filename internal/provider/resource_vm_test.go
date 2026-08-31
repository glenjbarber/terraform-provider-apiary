package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// fakeRestshim is a minimal in-memory stand-in for internal/restshim's
// real REST API, just enough to exercise the provider's full Terraform
// apply/refresh/destroy lifecycle without a real Apiary cluster - the
// same "fake the backend, not the CLI" approach used throughout this
// test file, mirroring how Apiary's own reconciler/frontend tests use
// fakes rather than real FreeBSD tooling.
type fakeRestshim struct {
	mu       sync.Mutex
	vms      map[string]VM
	jails    map[string]Jail
	networks map[string]Network
}

func newFakeRestshim() *fakeRestshim {
	return &fakeRestshim{
		vms:      map[string]VM{},
		jails:    map[string]Jail{},
		networks: map[string]Network{},
	}
}

func (f *fakeRestshim) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/vms", func(w http.ResponseWriter, r *http.Request) {
		var v VM
		json.NewDecoder(r.Body).Decode(&v)
		f.mu.Lock()
		defer f.mu.Unlock()
		if _, exists := f.vms[v.ID]; exists {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(apiErrorBody{Error: fmt.Sprintf("CreateVM: id %q already exists", v.ID)})
			return
		}
		f.vms[v.ID] = v
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(v)
	})
	mux.HandleFunc("GET /v1/vms/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		v, ok := f.vms[r.PathValue("id")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(v)
	})
	mux.HandleFunc("PUT /v1/vms/{id}", func(w http.ResponseWriter, r *http.Request) {
		var v VM
		json.NewDecoder(r.Body).Decode(&v)
		v.ID = r.PathValue("id")
		f.mu.Lock()
		defer f.mu.Unlock()
		f.vms[v.ID] = v
		json.NewEncoder(w).Encode(v)
	})
	mux.HandleFunc("DELETE /v1/vms/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		v := f.vms[r.PathValue("id")]
		delete(f.vms, r.PathValue("id"))
		json.NewEncoder(w).Encode(v)
	})

	mux.HandleFunc("POST /v1/jails", func(w http.ResponseWriter, r *http.Request) {
		var j Jail
		json.NewDecoder(r.Body).Decode(&j)
		f.mu.Lock()
		defer f.mu.Unlock()
		if _, exists := f.jails[j.ID]; exists {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(apiErrorBody{Error: fmt.Sprintf("CreateJail: id %q already exists", j.ID)})
			return
		}
		f.jails[j.ID] = j
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(j)
	})
	mux.HandleFunc("GET /v1/jails/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		j, ok := f.jails[r.PathValue("id")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(j)
	})
	mux.HandleFunc("PUT /v1/jails/{id}", func(w http.ResponseWriter, r *http.Request) {
		var j Jail
		json.NewDecoder(r.Body).Decode(&j)
		j.ID = r.PathValue("id")
		f.mu.Lock()
		defer f.mu.Unlock()
		f.jails[j.ID] = j
		json.NewEncoder(w).Encode(j)
	})
	mux.HandleFunc("DELETE /v1/jails/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		j := f.jails[r.PathValue("id")]
		delete(f.jails, r.PathValue("id"))
		json.NewEncoder(w).Encode(j)
	})

	mux.HandleFunc("POST /v1/networks", func(w http.ResponseWriter, r *http.Request) {
		var n Network
		json.NewDecoder(r.Body).Decode(&n)
		f.mu.Lock()
		defer f.mu.Unlock()
		if _, exists := f.networks[n.ID]; exists {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(apiErrorBody{Error: fmt.Sprintf("CreateNetwork: id %q already exists", n.ID)})
			return
		}
		f.networks[n.ID] = n
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(n)
	})
	mux.HandleFunc("GET /v1/networks", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := make([]Network, 0, len(f.networks))
		for _, n := range f.networks {
			out = append(out, n)
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("DELETE /v1/networks/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		n := f.networks[r.PathValue("id")]
		delete(f.networks, r.PathValue("id"))
		json.NewEncoder(w).Encode(n)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func providerConfig(endpoint string) string {
	return fmt.Sprintf(`
provider "apiary" {
  endpoint = %q
}
`, endpoint)
}

func TestAccVMResource_CreateUpdateDestroy(t *testing.T) {
	backend := newFakeRestshim()
	srv := backend.server(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_vm" "test" {
  id            = "vm-1"
  name          = "web-1"
  vcpus         = 2
  memory_mb     = 1024
  node_id       = "node-a"
  desired_state = "running"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apiary_vm.test", "id", "vm-1"),
					resource.TestCheckResourceAttr("apiary_vm.test", "name", "web-1"),
					resource.TestCheckResourceAttr("apiary_vm.test", "vcpus", "2"),
					resource.TestCheckResourceAttr("apiary_vm.test", "memory_mb", "1024"),
					resource.TestCheckResourceAttr("apiary_vm.test", "node_id", "node-a"),
					resource.TestCheckResourceAttr("apiary_vm.test", "desired_state", "running"),
				),
			},
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_vm" "test" {
  id            = "vm-1"
  name          = "web-1-renamed"
  vcpus         = 4
  memory_mb     = 2048
  node_id       = "node-a"
  desired_state = "stopped"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("apiary_vm.test", "name", "web-1-renamed"),
					resource.TestCheckResourceAttr("apiary_vm.test", "vcpus", "4"),
					resource.TestCheckResourceAttr("apiary_vm.test", "memory_mb", "2048"),
					resource.TestCheckResourceAttr("apiary_vm.test", "desired_state", "stopped"),
				),
			},
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_vm" "test" {
  id            = "vm-1"
  name          = "web-1-renamed"
  vcpus         = 4
  memory_mb     = 2048
  node_id       = "node-a"
  desired_state = "stopped"
}
`,
				ResourceName:      "apiary_vm.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccVMResource_ReplicaNodeID(t *testing.T) {
	backend := newFakeRestshim()
	srv := backend.server(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv.URL) + `
resource "apiary_vm" "test" {
  id              = "vm-1"
  node_id         = "node-a"
  replica_node_id = "node-b"
}
`,
				Check: resource.TestCheckResourceAttr("apiary_vm.test", "replica_node_id", "node-b"),
			},
		},
	})
}
