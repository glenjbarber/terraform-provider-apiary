package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &NetworkResource{}
	_ resource.ResourceWithConfigure   = &NetworkResource{}
	_ resource.ResourceWithImportState = &NetworkResource{}
)

func NewNetworkResource() resource.Resource {
	return &NetworkResource{}
}

// NetworkResource manages an Apiary NetworkDefinition. Unlike VM/Jail,
// Apiary's ManagerService has no UpdateNetwork or GetNetwork(id) RPC
// (only CreateNetwork/ListNetworks/DeleteNetwork exist - see
// api/rpc/manager.proto), so every attribute here requires replacement
// on change: there is no in-place update to translate to.
type NetworkResource struct {
	client *Client
}

type networkResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	VLANID       types.Int64  `tfsdk:"vlan_id"`
	Subnet       types.String `tfsdk:"subnet"`
	BridgeName   types.String `tfsdk:"bridge_name"`
	BridgeStatus types.String `tfsdk:"bridge_status"`
}

func (r *NetworkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (r *NetworkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Apiary network (a named, cluster-wide VLAN + subnet a VM can attach to). Every attribute requires replacement on change - Apiary has no update-in-place RPC for networks.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "Caller-chosen network identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Display name for the network.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"vlan_id": schema.Int64Attribute{
				Optional:    true,
				Description: "802.1Q VLAN tag, or 0/unset for an untagged network sharing the uplink's own L2 segment.",
			},
			"subnet": schema.StringAttribute{
				Required:    true,
				Description: "CIDR, e.g. \"10.60.0.0/24\". Apiary allocates VM IPs from this range.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bridge_name": schema.StringAttribute{
				Optional:    true,
				Description: "Override for the bridge(4) interface name the reconciler uses on every node (default: a short hash of id). No reason to need this in practice.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"bridge_status": schema.StringAttribute{
				Computed:    true,
				Description: "Physical, per-node bridge status (\"up\"/\"down\"/\"unknown\") as last reported by ListNetworks - read-only.",
			},
		},
	}
}

func (r *NetworkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected resource configure type", fmt.Sprintf("expected *provider.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func networkToModel(n *Network) networkResourceModel {
	m := networkResourceModel{
		ID:     types.StringValue(n.ID),
		Subnet: types.StringValue(n.Subnet),
	}
	// 0 means "untagged" for a real VLAN ID (a legitimate value, not
	// just "unset"), but it also round-trips through restshim's
	// "omitempty" JSON tag identically to a caller who never set
	// vlan_id at all - see vmToModel's own doc comment for why treating
	// 0 as null here avoids a "provider produced inconsistent result"
	// error when vlan_id is left out of the config entirely.
	if n.VLANID != 0 {
		m.VLANID = types.Int64Value(int64(n.VLANID))
	} else {
		m.VLANID = types.Int64Null()
	}
	if n.Name != "" {
		m.Name = types.StringValue(n.Name)
	} else {
		m.Name = types.StringNull()
	}
	if n.BridgeName != "" {
		m.BridgeName = types.StringValue(n.BridgeName)
	} else {
		m.BridgeName = types.StringNull()
	}
	if n.BridgeStatus != "" {
		m.BridgeStatus = types.StringValue(n.BridgeStatus)
	} else {
		m.BridgeStatus = types.StringValue("unknown")
	}
	return m
}

func modelToNetwork(m networkResourceModel) Network {
	return Network{
		ID:         m.ID.ValueString(),
		Name:       m.Name.ValueString(),
		VLANID:     uint32(m.VLANID.ValueInt64()),
		Subnet:     m.Subnet.ValueString(),
		BridgeName: m.BridgeName.ValueString(),
	}
}

func (r *NetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateNetwork(ctx, modelToNetwork(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating network", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, networkToModel(created))...)
}

func (r *NetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	network, err := r.client.GetNetwork(ctx, state.ID.ValueString())
	if errors.Is(err, ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading network", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, networkToModel(network))...)
}

// Update is unreachable in practice - every attribute that could
// change is marked RequiresReplace above, so Terraform always plans a
// destroy/create instead of calling this. Implemented anyway (returning
// a clear error rather than silently no-opping) since
// resource.Resource requires an Update method regardless.
func (r *NetworkResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"apiary_network has no in-place update path (Apiary's ManagerService has no UpdateNetwork RPC) - every attribute requires replacement, so this should be unreachable. If you see this, please report it as a provider bug.",
	)
}

func (r *NetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteNetwork(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting network", err.Error())
	}
}

func (r *NetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
