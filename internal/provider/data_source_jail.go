package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var (
	_ datasource.DataSource              = &JailDataSource{}
	_ datasource.DataSourceWithConfigure = &JailDataSource{}
)

func NewJailDataSource() datasource.DataSource {
	return &JailDataSource{}
}

// JailDataSource looks up an existing Apiary jail by id - mirrors
// VMDataSource's own reasoning.
type JailDataSource struct {
	client *Client
}

func (d *JailDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jail"
}

func (d *JailDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up an existing Apiary jail by id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "Jail identifier to look up.",
			},
			"name":            schema.StringAttribute{Computed: true, Description: "Display name for the jail."},
			"hostname":        schema.StringAttribute{Computed: true, Description: "The jail's own hostname."},
			"node_id":         schema.StringAttribute{Computed: true, Description: "Cluster node this jail is assigned to run on."},
			"replica_node_id": schema.StringAttribute{Computed: true, Description: "A second cluster node HAST-replicating this jail's root filesystem, if any."},
			"desired_state":   schema.StringAttribute{Computed: true, Description: "\"running\" or \"stopped\"."},
		},
	}
}

func (d *JailDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *JailDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config jailResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	jail, err := d.client.GetJail(ctx, config.ID.ValueString())
	if errors.Is(err, ErrNotFound) {
		resp.Diagnostics.AddError("Jail not found", fmt.Sprintf("no jail with id %q exists", config.ID.ValueString()))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading jail", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, jailToModel(jail))...)
}
