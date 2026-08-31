package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var (
	_ datasource.DataSource              = &NetworkDataSource{}
	_ datasource.DataSourceWithConfigure = &NetworkDataSource{}
)

func NewNetworkDataSource() datasource.DataSource {
	return &NetworkDataSource{}
}

// NetworkDataSource looks up an existing Apiary network by id - mirrors
// VMDataSource's own reasoning.
type NetworkDataSource struct {
	client *Client
}

func (d *NetworkDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (d *NetworkDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up an existing Apiary network by id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "Network identifier to look up.",
			},
			"name":          schema.StringAttribute{Computed: true, Description: "Display name for the network."},
			"vlan_id":       schema.Int64Attribute{Computed: true, Description: "VLAN tag, if any (0/unset means untagged)."},
			"subnet":        schema.StringAttribute{Computed: true, Description: "The network's subnet, e.g. \"10.50.1.0/24\"."},
			"bridge_name":   schema.StringAttribute{Computed: true, Description: "The bridge(4) interface name backing this network."},
			"bridge_status": schema.StringAttribute{Computed: true, Description: "\"up\" or \"down\" - this network's real per-node bridge status."},
		},
	}
}

func (d *NetworkDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NetworkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config networkResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	network, err := d.client.GetNetwork(ctx, config.ID.ValueString())
	if errors.Is(err, ErrNotFound) {
		resp.Diagnostics.AddError("Network not found", fmt.Sprintf("no network with id %q exists", config.ID.ValueString()))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading network", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, networkToModel(network))...)
}
