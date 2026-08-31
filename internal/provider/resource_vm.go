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
	_ resource.Resource                = &VMResource{}
	_ resource.ResourceWithConfigure   = &VMResource{}
	_ resource.ResourceWithImportState = &VMResource{}
)

func NewVMResource() resource.Resource {
	return &VMResource{}
}

// VMResource manages an Apiary VM - a real, replicated, persisted
// VMDefinition (see apiary's CLAUDE.md). id is caller-chosen, exactly
// like every other identifier in Apiary's own data model (no
// server-generated ids to reconcile with Terraform's own id field).
type VMResource struct {
	client *Client
}

// vmResourceModel is the Terraform-facing shape of an apiary_vm
// resource block, mirroring provider.Client's VM type field-for-field.
type vmResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	VCPUs         types.Int64  `tfsdk:"vcpus"`
	MemoryMB      types.Int64  `tfsdk:"memory_mb"`
	NodeID        types.String `tfsdk:"node_id"`
	DesiredState  types.String `tfsdk:"desired_state"`
	ReplicaNodeID types.String `tfsdk:"replica_node_id"`
	ISOName       types.String `tfsdk:"iso_name"`
	NetworkID     types.String `tfsdk:"network_id"`
	BaseImageName types.String `tfsdk:"base_image_name"`
	IPAddress     types.String `tfsdk:"ip_address"`
	MACAddress    types.String `tfsdk:"mac_address"`
}

func (r *VMResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm"
}

func (r *VMResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Apiary VM.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "Caller-chosen VM identifier. Apiary has no automatic id generation - this is the id used everywhere else in Apiary (the web UI, restshim, managerd's RPC API).",
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Display name for the VM.",
			},
			"vcpus": schema.Int64Attribute{
				Optional:    true,
				Description: "Number of virtual CPUs. Apiary defaults to 1 if unset.",
			},
			"memory_mb": schema.Int64Attribute{
				Optional:    true,
				Description: "Memory in MB. Apiary defaults to 512 if unset.",
			},
			"node_id": schema.StringAttribute{
				Optional:    true,
				Description: "Cluster node this VM is assigned to run on. Apiary has no automatic scheduling (see its CLAUDE.md) - this must name a real raft cluster member for the VM to actually be provisioned anywhere.",
			},
			"desired_state": schema.StringAttribute{
				Optional:    true,
				Description: "\"running\" or \"stopped\". Unset means Apiary's own default (VM_STATE_UNSPECIFIED).",
			},
			"replica_node_id": schema.StringAttribute{
				Optional:    true,
				Description: "A second cluster node to HAST-replicate this VM's disk to, for data redundancy (not automatic failover). Caller-set, exactly like node_id - see Apiary's ADR-0026.",
			},
			"iso_name": schema.StringAttribute{
				Optional:    true,
				Description: "Name of an image already uploaded to the assigned node via Apiary's UploadISO RPC - attached as a CD-ROM device. See Apiary's ADR-0017.",
			},
			"network_id": schema.StringAttribute{
				Optional:    true,
				Description: "An Apiary NetworkDefinition (see apiary_network) this VM's NIC belongs to. Leave unset for Apiary's flat, node-wide bridge behavior. See Apiary's ADR-0022.",
			},
			"base_image_name": schema.StringAttribute{
				Optional:    true,
				Description: "Name of a raw disk image already uploaded via UploadISO - the VM's disk is seeded from it on first creation instead of created blank. Required in practice for an unattended Linux VM to boot into a working OS. See Apiary's ADR-0031.",
			},
			"ip_address": schema.StringAttribute{
				Computed:    true,
				Description: "IP address Apiary's raft FSM assigned this VM when network_id is set - read-only, never caller-set.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mac_address": schema.StringAttribute{
				Computed:    true,
				Description: "MAC address Apiary's raft FSM assigned this VM when network_id is set - read-only, never caller-set.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *VMResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func vmToModel(v *VM) vmResourceModel {
	m := vmResourceModel{ID: types.StringValue(v.ID)}
	// VCPUs/MemoryMB round-trip through restshim's own "omitempty" JSON
	// tags, so a real zero and "the caller never set this, Apiary
	// applies its own default" are indistinguishable on the wire - both
	// arrive here as 0. Treating 0 as null (rather than
	// Int64Value(0)) keeps a config that never set these attributes
	// consistent after apply instead of Terraform reporting "provider
	// produced inconsistent result" for turning a null into a literal 0.
	if v.VCPUs != 0 {
		m.VCPUs = types.Int64Value(int64(v.VCPUs))
	} else {
		m.VCPUs = types.Int64Null()
	}
	if v.MemoryMB != 0 {
		m.MemoryMB = types.Int64Value(int64(v.MemoryMB))
	} else {
		m.MemoryMB = types.Int64Null()
	}
	if v.Name != "" {
		m.Name = types.StringValue(v.Name)
	} else {
		m.Name = types.StringNull()
	}
	if v.NodeID != "" {
		m.NodeID = types.StringValue(v.NodeID)
	} else {
		m.NodeID = types.StringNull()
	}
	if v.DesiredState != "" {
		m.DesiredState = types.StringValue(v.DesiredState)
	} else {
		m.DesiredState = types.StringNull()
	}
	if v.ReplicaNodeID != "" {
		m.ReplicaNodeID = types.StringValue(v.ReplicaNodeID)
	} else {
		m.ReplicaNodeID = types.StringNull()
	}
	if v.ISOName != "" {
		m.ISOName = types.StringValue(v.ISOName)
	} else {
		m.ISOName = types.StringNull()
	}
	if v.NetworkID != "" {
		m.NetworkID = types.StringValue(v.NetworkID)
	} else {
		m.NetworkID = types.StringNull()
	}
	if v.BaseImageName != "" {
		m.BaseImageName = types.StringValue(v.BaseImageName)
	} else {
		m.BaseImageName = types.StringNull()
	}
	// IPAddress/MACAddress are read-only, assigned by Apiary's FSM only
	// when network_id is set - null (not empty-string) when unassigned,
	// matching every other optional field's null-vs-zero handling above.
	if v.IPAddress != "" {
		m.IPAddress = types.StringValue(v.IPAddress)
	} else {
		m.IPAddress = types.StringNull()
	}
	if v.MACAddress != "" {
		m.MACAddress = types.StringValue(v.MACAddress)
	} else {
		m.MACAddress = types.StringNull()
	}
	return m
}

func modelToVM(m vmResourceModel) VM {
	return VM{
		ID:            m.ID.ValueString(),
		Name:          m.Name.ValueString(),
		VCPUs:         uint32(m.VCPUs.ValueInt64()),
		MemoryMB:      uint64(m.MemoryMB.ValueInt64()),
		NodeID:        m.NodeID.ValueString(),
		DesiredState:  m.DesiredState.ValueString(),
		ReplicaNodeID: m.ReplicaNodeID.ValueString(),
		ISOName:       m.ISOName.ValueString(),
		NetworkID:     m.NetworkID.ValueString(),
		BaseImageName: m.BaseImageName.ValueString(),
	}
}

func (r *VMResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vmResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateVM(ctx, modelToVM(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating VM", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, vmToModel(created))...)
}

func (r *VMResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vmResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vm, err := r.client.GetVM(ctx, state.ID.ValueString())
	if errors.Is(err, ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading VM", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, vmToModel(vm))...)
}

func (r *VMResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan vmResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateVM(ctx, plan.ID.ValueString(), modelToVM(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating VM", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, vmToModel(updated))...)
}

func (r *VMResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteVM(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting VM", err.Error())
	}
}

func (r *VMResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
