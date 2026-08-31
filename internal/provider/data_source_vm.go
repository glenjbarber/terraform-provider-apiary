package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var (
	_ datasource.DataSource              = &VMDataSource{}
	_ datasource.DataSourceWithConfigure = &VMDataSource{}
)

func NewVMDataSource() datasource.DataSource {
	return &VMDataSource{}
}

// VMDataSource looks up an existing Apiary VM by id - for referencing a
// VM Terraform doesn't manage (created by hand, by another tool, or by
// a separate Terraform state) rather than importing it into this
// resource's own lifecycle.
type VMDataSource struct {
	client *Client
}

func (d *VMDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm"
}

func (d *VMDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up an existing Apiary VM by id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "VM identifier to look up.",
			},
			"name":            schema.StringAttribute{Computed: true, Description: "Display name for the VM."},
			"vcpus":           schema.Int64Attribute{Computed: true, Description: "Number of virtual CPUs."},
			"memory_mb":       schema.Int64Attribute{Computed: true, Description: "Memory in MB."},
			"node_id":         schema.StringAttribute{Computed: true, Description: "Cluster node this VM is assigned to run on."},
			"desired_state":   schema.StringAttribute{Computed: true, Description: "\"running\" or \"stopped\"."},
			"replica_node_id": schema.StringAttribute{Computed: true, Description: "A second cluster node HAST-replicating this VM's disk, if any."},
			"iso_name":        schema.StringAttribute{Computed: true, Description: "Attached installer/seed image name, if any."},
			"network_id":      schema.StringAttribute{Computed: true, Description: "NetworkDefinition this VM's NIC belongs to, if any."},
			"base_image_name": schema.StringAttribute{Computed: true, Description: "Base disk image this VM's disk was seeded from, if any. See Apiary's ADR-0031."},
			"ip_address":      schema.StringAttribute{Computed: true, Description: "IP address Apiary's raft FSM assigned this VM, if network_id is set."},
			"mac_address":     schema.StringAttribute{Computed: true, Description: "MAC address Apiary's raft FSM assigned this VM, if network_id is set."},
		},
	}
}

func (d *VMDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected data source configure type", fmt.Sprintf("expected *provider.Client, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *VMDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config vmResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vm, err := d.client.GetVM(ctx, config.ID.ValueString())
	if errors.Is(err, ErrNotFound) {
		resp.Diagnostics.AddError("VM not found", fmt.Sprintf("no VM with id %q exists", config.ID.ValueString()))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading VM", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, vmToModel(vm))...)
}
