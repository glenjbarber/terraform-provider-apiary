package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "test-key", srv.Client()), srv
}

func TestClient_CreateVM_ForwardsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(VM{ID: "vm-1"})
	})

	if _, err := client.CreateVM(context.Background(), VM{ID: "vm-1"}); err != nil {
		t.Fatalf("CreateVM() error: %v", err)
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization header = %q, want \"Bearer test-key\"", gotAuth)
	}
}

func TestClient_CreateVM_Success(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/vms" {
			t.Errorf("request = %s %s, want POST /v1/vms", r.Method, r.URL.Path)
		}
		var body VM
		json.NewDecoder(r.Body).Decode(&body)
		if body.ID != "vm-1" {
			t.Errorf("request body id = %q, want vm-1", body.ID)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(VM{ID: "vm-1", Name: "web-1"})
	})

	vm, err := client.CreateVM(context.Background(), VM{ID: "vm-1"})
	if err != nil {
		t.Fatalf("CreateVM() error: %v", err)
	}
	if vm.Name != "web-1" {
		t.Errorf("VM.Name = %q, want web-1", vm.Name)
	}
}

func TestClient_CreateVM_ApplicationErrorIsReturned(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(apiErrorBody{Error: `CreateVM: id "vm-1" already exists`})
	})

	_, err := client.CreateVM(context.Background(), VM{ID: "vm-1"})
	if err == nil {
		t.Fatal("CreateVM() error = nil, want the application error")
	}
}

func TestClient_GetVM_NotFoundReturnsErrNotFound(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetVM(context.Background(), "vm-missing")
	if err != ErrNotFound {
		t.Errorf("GetVM() error = %v, want ErrNotFound", err)
	}
}

func TestClient_DeleteVM_NotFoundIsNotAnError(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	if err := client.DeleteVM(context.Background(), "vm-missing"); err != nil {
		t.Errorf("DeleteVM() error = %v, want nil (already gone is idempotent)", err)
	}
}

func TestClient_UpdateVM_UsesPathID(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/vms/vm-1" {
			t.Errorf("request = %s %s, want PUT /v1/vms/vm-1", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(VM{ID: "vm-1", Name: "renamed"})
	})

	vm, err := client.UpdateVM(context.Background(), "vm-1", VM{ID: "vm-1", Name: "renamed"})
	if err != nil {
		t.Fatalf("UpdateVM() error: %v", err)
	}
	if vm.Name != "renamed" {
		t.Errorf("VM.Name = %q, want renamed", vm.Name)
	}
}

func TestClient_CreateJail_Success(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/jails" {
			t.Errorf("request = %s %s, want POST /v1/jails", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Jail{ID: "jail-1", Name: "web-1"})
	})

	jail, err := client.CreateJail(context.Background(), Jail{ID: "jail-1"})
	if err != nil {
		t.Fatalf("CreateJail() error: %v", err)
	}
	if jail.Name != "web-1" {
		t.Errorf("Jail.Name = %q, want web-1", jail.Name)
	}
}

func TestClient_GetJail_NotFoundReturnsErrNotFound(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.GetJail(context.Background(), "jail-missing")
	if err != ErrNotFound {
		t.Errorf("GetJail() error = %v, want ErrNotFound", err)
	}
}

func TestClient_DeleteJail_NotFoundIsNotAnError(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	if err := client.DeleteJail(context.Background(), "jail-missing"); err != nil {
		t.Errorf("DeleteJail() error = %v, want nil", err)
	}
}

func TestClient_CreateNetwork_Success(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/networks" {
			t.Errorf("request = %s %s, want POST /v1/networks", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Network{ID: "net-1", Subnet: "10.60.0.0/24"})
	})

	network, err := client.CreateNetwork(context.Background(), Network{ID: "net-1", Subnet: "10.60.0.0/24"})
	if err != nil {
		t.Fatalf("CreateNetwork() error: %v", err)
	}
	if network.Subnet != "10.60.0.0/24" {
		t.Errorf("Network.Subnet = %q, want 10.60.0.0/24", network.Subnet)
	}
}

// GetNetwork has no direct REST endpoint (see its own doc comment) -
// it must fall back to ListNetworks and find the match client-side.
func TestClient_GetNetwork_FallsBackToListNetworks(t *testing.T) {
	var gotPath string
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode([]Network{
			{ID: "net-1", Subnet: "10.60.0.0/24"},
			{ID: "net-2", Subnet: "10.61.0.0/24"},
		})
	})

	network, err := client.GetNetwork(context.Background(), "net-2")
	if err != nil {
		t.Fatalf("GetNetwork() error: %v", err)
	}
	if gotPath != "/v1/networks" {
		t.Errorf("request path = %q, want /v1/networks", gotPath)
	}
	if network.Subnet != "10.61.0.0/24" {
		t.Errorf("Network.Subnet = %q, want 10.61.0.0/24", network.Subnet)
	}
}

func TestClient_GetNetwork_NotInListIsErrNotFound(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Network{{ID: "net-1", Subnet: "10.60.0.0/24"}})
	})

	_, err := client.GetNetwork(context.Background(), "net-missing")
	if err != ErrNotFound {
		t.Errorf("GetNetwork() error = %v, want ErrNotFound", err)
	}
}

func TestClient_DeleteNetwork_NotFoundIsNotAnError(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	if err := client.DeleteNetwork(context.Background(), "net-missing"); err != nil {
		t.Errorf("DeleteNetwork() error = %v, want nil", err)
	}
}

func TestClient_LeaderHintIsIncludedInError(t *testing.T) {
	client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(apiErrorBody{Error: "not leader", LeaderHint: "10.0.0.2:17600"})
	})

	_, err := client.CreateVM(context.Background(), VM{ID: "vm-1"})
	if err == nil {
		t.Fatal("CreateVM() error = nil, want the leader-hint error")
	}
	if got := err.Error(); got != "not leader (leader hint: 10.0.0.2:17600)" {
		t.Errorf("error = %q, want to include the leader hint", got)
	}
}

// Apiary's REST shim returns 400 (not 404) for an absent ID rejected
// by the raft FSM. Suppress only the exact matching DELETE error.
func TestClient_DeleteMissingIDAndOtherBadRequests(t *testing.T) {
	cases := []struct {
		name, operation, id, path string
		deleteFn func(*Client, string) error
	}{
		{"vm", "DeleteVM", "vm-missing", "/v1/vms/vm-missing", (*Client).DeleteVM},
		{"jail", "DeleteJail", "jail-missing", "/v1/jails/jail-missing", (*Client).DeleteJail},
		{"network", "DeleteNetwork", "net-missing", "/v1/networks/net-missing", (*Client).DeleteNetwork},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := ""
			status := http.StatusBadRequest
			client, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != tc.path {
					t.Errorf("request = %s %s, want DELETE %s", r.Method, r.URL.Path, tc.path)
				}
				w.WriteHeader(status)
				json.NewEncoder(w).Encode(apiErrorBody{Error: message})
			})
			message = tc.operation + ": id \\"" + tc.id + "\\" does not exist"
			if err := tc.deleteFn(client, tc.id); err != nil {
				t.Errorf("missing ID error = %v, want nil", err)
			}
			for _, bad := range []string{
				tc.operation + ": still referenced by another resource",
				"DeleteVM: id \\"" + tc.id + "\\" does not exist",
				tc.operation + ": id \\"other\\" does not exist",
			} {
				if bad == message { continue }
				message = bad
				if err := tc.deleteFn(client, tc.id); err == nil {
					t.Errorf("unexpectedly suppressed HTTP 400: %q", bad)
				}
			}
			message = tc.operation + ": id \\"" + tc.id + "\\" does not exist"
			status = http.StatusServiceUnavailable
			if err := tc.deleteFn(client, tc.id); err == nil {
				t.Error("unexpectedly suppressed HTTP 503")
			}
		})
	}
}
