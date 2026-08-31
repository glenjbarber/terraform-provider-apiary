// Package provider implements the Terraform provider for Apiary. It
// talks to Apiary's restshimd REST API (see internal/restshim in the
// apiary repo) rather than managerd's gRPC API directly - restshimd is
// the piece of Apiary's own architecture built specifically to sit in
// front of external tooling like this, forwarding each caller's own
// Authorization header straight through to managerd (see apiary's
// ADR-0023/ADR-0024).
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ErrNotFound is returned by Get* methods when restshimd reports a 404
// for the requested id - a normal, expected outcome (e.g. a resource
// deleted outside Terraform), not a client bug.
var ErrNotFound = errors.New("apiary: resource not found")

// Client is a thin REST client for restshimd's /v1 API. It has no
// retry/backoff logic of its own - Terraform's own apply/refresh cycle
// is the retry boundary here, matching how thin apiary's own restshim
// clients (cmd/frontend, cmd/restshimd itself) are kept.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient returns a Client targeting baseURL (e.g.
// "http://localhost:8080"), authenticating with apiKey (may be empty,
// matching managerd's own "zero keys means unauthenticated" behavior -
// see apiary's ADR-0023).
func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, http: httpClient}
}

// apiErrorBody mirrors restshim's own errorBody JSON shape.
type apiErrorBody struct {
	Error      string `json:"error"`
	LeaderHint string `json:"leader_hint,omitempty"`
}

func (e apiErrorBody) asError() error {
	if e.LeaderHint != "" {
		return fmt.Errorf("%s (leader hint: %s)", e.Error, e.LeaderHint)
	}
	return errors.New(e.Error)
}

// do issues one HTTP request against path, marshaling body as the
// request JSON (if non-nil) and unmarshaling the response into out (if
// non-nil). A 404 response is reported as ErrNotFound specifically so
// callers (Terraform's Read methods) can distinguish "gone" from every
// other failure and remove the resource from state instead of erroring.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("apiary: encoding request body: %w", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("apiary: building request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("apiary: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiErrorBody
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error == "" {
			return fmt.Errorf("apiary: %s %s: unexpected status %d", method, path, resp.StatusCode)
		}
		return apiErr.asError()
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("apiary: decoding response: %w", err)
		}
	}
	return nil
}

// VM mirrors internal/restshim's own vm JSON shape - including the
// iso_name/network_id/ip_address/mac_address/base_image_name fields
// restshim gained alongside Apiary's ADR-0031 (VM base disk images),
// previously missing from this client entirely.
type VM struct {
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	VCPUs         uint32 `json:"vcpus,omitempty"`
	MemoryMB      uint64 `json:"memory_mb,omitempty"`
	NodeID        string `json:"node_id,omitempty"`
	DesiredState  string `json:"desired_state,omitempty"`
	ReplicaNodeID string `json:"replica_node_id,omitempty"`
	ISOName       string `json:"iso_name,omitempty"`
	NetworkID     string `json:"network_id,omitempty"`
	IPAddress     string `json:"ip_address,omitempty"`
	MACAddress    string `json:"mac_address,omitempty"`
	BaseImageName string `json:"base_image_name,omitempty"`
}

func (c *Client) CreateVM(ctx context.Context, v VM) (*VM, error) {
	var out VM
	if err := c.do(ctx, http.MethodPost, "/v1/vms", v, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetVM(ctx context.Context, id string) (*VM, error) {
	var out VM
	if err := c.do(ctx, http.MethodGet, "/v1/vms/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateVM(ctx context.Context, id string, v VM) (*VM, error) {
	var out VM
	if err := c.do(ctx, http.MethodPut, "/v1/vms/"+id, v, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteVM(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/v1/vms/"+id, nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil // already gone - Delete is idempotent from Terraform's perspective
	}
	return err
}

// Jail mirrors internal/restshim's own jail JSON shape.
type Jail struct {
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	Hostname      string `json:"hostname,omitempty"`
	NodeID        string `json:"node_id,omitempty"`
	ReplicaNodeID string `json:"replica_node_id,omitempty"`
	DesiredState  string `json:"desired_state,omitempty"`
}

func (c *Client) CreateJail(ctx context.Context, j Jail) (*Jail, error) {
	var out Jail
	if err := c.do(ctx, http.MethodPost, "/v1/jails", j, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetJail(ctx context.Context, id string) (*Jail, error) {
	var out Jail
	if err := c.do(ctx, http.MethodGet, "/v1/jails/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateJail(ctx context.Context, id string, j Jail) (*Jail, error) {
	var out Jail
	if err := c.do(ctx, http.MethodPut, "/v1/jails/"+id, j, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteJail(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/v1/jails/"+id, nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// Network mirrors internal/restshim's own network JSON shape.
type Network struct {
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	VLANID       uint32 `json:"vlan_id,omitempty"`
	Subnet       string `json:"subnet"`
	BridgeName   string `json:"bridge_name,omitempty"`
	BridgeStatus string `json:"bridge_status,omitempty"`
}

func (c *Client) CreateNetwork(ctx context.Context, n Network) (*Network, error) {
	var out Network
	if err := c.do(ctx, http.MethodPost, "/v1/networks", n, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetNetwork has no direct REST endpoint (restshim has no per-id
// network route - see its own doc comment on why: NetworkDefinition
// has no external Get-by-id RPC to translate), so this lists every
// network and finds the matching id client-side. Fine for the small
// network counts a real deployment has; not something worth adding
// server-side pagination for at this scale.
func (c *Client) GetNetwork(ctx context.Context, id string) (*Network, error) {
	networks, err := c.ListNetworks(ctx)
	if err != nil {
		return nil, err
	}
	for _, n := range networks {
		if n.ID == id {
			return &n, nil
		}
	}
	return nil, ErrNotFound
}

func (c *Client) ListNetworks(ctx context.Context) ([]Network, error) {
	var out []Network
	if err := c.do(ctx, http.MethodGet, "/v1/networks", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeleteNetwork(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/v1/networks/"+id, nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
