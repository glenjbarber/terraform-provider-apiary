package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &JailResource{}
	_ resource.ResourceWithConfigure   = &JailResource{}
	_ resource.ResourceWithImportState = &JailResource{}
)

func NewJailResource() resource.Resource {
	return &JailResource{}
}

// JailResource manages an Apiary jail - deliberately minimal like
// JailDefinition itself (see Apiary's ADR-0027): no vcpus/memory/
// network fields, since Apiary's jail orchestration v1 scope is flat
// ip4=inherit networking with no dedicated resource limits.
type JailResource struct {
	client *Client
}

type jailResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Hostname      types.String `tfsdk:"hostname"`
	NodeID        types.String `tfsdk:"node_id"`
	ReplicaNodeID types.String `tfsdk:"replica_node_id"`
	DesiredState  types.String `tfsdk:"desired_state"`
}

func (r *JailResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jail"
}

func (r *JailResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an Apiary jail.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "Caller-chosen jail identifier.",
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Description: "Display name for the jail.",
			},
			"hostname": schema.StringAttribute{
				Optional:    true,
				Description: "Passed to jail(8) as the jail's own hostname.",
			},
			"node_id": schema.StringAttribute{
				Optional:    true,
				Description: "Cluster node this jail is assigned to run on. Apiary has no automatic scheduling - this must name a real raft cluster member for the jail to actually be provisioned anywhere.",
			},
			"replica_node_id": schema.StringAttribute{
				Optional:    true,
				Description: "A second cluster node to HAST-replicate this jail's root filesystem to, for data redundancy (not automatic failover). See Apiary's ADR-0027.",
			},
			"desired_state": schema.StringAttribute{
				Optional:    true,
				Description: "\"running\" or \"stopped\". Unset means Apiary's own default (JAIL_STATE_UNSPECIFIED).",
			},
		},
	}
}

func (r *JailResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func jailToModel(j *Jail) jailResourceModel {
	m := jailResourceModel{ID: types.StringValue(j.ID)}
	if j.Name != "" {
		m.Name = types.StringValue(j.Name)
	} else {
		m.Name = types.StringNull()
	}
	if j.Hostname != "" {
		m.Hostname = types.StringValue(j.Hostname)
	} else {
		m.Hostname = types.StringNull()
	}
	if j.NodeID != "" {
		m.NodeID = types.StringValue(j.NodeID)
	} else {
		m.NodeID = types.StringNull()
	}
	if j.ReplicaNodeID != "" {
		m.ReplicaNodeID = types.StringValue(j.ReplicaNodeID)
	} else {
		m.ReplicaNodeID = types.StringNull()
	}
	if j.DesiredState != "" {
		m.DesiredState = types.StringValue(j.DesiredState)
	} else {
		m.DesiredState = types.StringNull()
	}
	return m
}

func modelToJail(m jailResourceModel) Jail {
	return Jail{
		ID:            m.ID.ValueString(),
		Name:          m.Name.ValueString(),
		Hostname:      m.Hostname.ValueString(),
		NodeID:        m.NodeID.ValueString(),
		ReplicaNodeID: m.ReplicaNodeID.ValueString(),
		DesiredState:  m.DesiredState.ValueString(),
	}
}

func (r *JailResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan jailResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateJail(ctx, modelToJail(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating jail", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, jailToModel(created))...)
}

func (r *JailResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state jailResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	jail, err := r.client.GetJail(ctx, state.ID.ValueString())
	if errors.Is(err, ErrNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading jail", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, jailToModel(jail))...)
}

func (r *JailResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan jailResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := r.client.UpdateJail(ctx, plan.ID.ValueString(), modelToJail(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error updating jail", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, jailToModel(updated))...)
}

func (r *JailResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state jailResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteJail(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting jail", err.Error())
	}
}

func (r *JailResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
