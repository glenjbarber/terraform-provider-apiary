package provider

import (
	"context"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ApiaryProvider satisfies the provider.Provider interface.
var _ provider.Provider = &ApiaryProvider{}

// ApiaryProvider is the Terraform provider for Apiary. version is
// injected by main.go at build time (via -ldflags), matching the
// standard terraform-plugin-framework scaffold's own pattern.
type ApiaryProvider struct {
	version string
}

// apiaryProviderModel is the provider's own configuration block:
//
//	provider "apiary" {
//	  endpoint = "http://localhost:8080"
//	  api_key  = "..."
//	}
type apiaryProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ApiaryProvider{version: version}
	}
}

func (p *ApiaryProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "apiary"
	resp.Version = p.version
}

func (p *ApiaryProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Apiary (a FreeBSD-native virtualization platform: bhyve VMs, jails, ZFS/HAST storage) through its restshimd REST API.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of Apiary's restshimd, e.g. \"http://localhost:8080\". Defaults to the APIARY_ENDPOINT environment variable.",
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Bearer API key for managerd's own auth (see Apiary's ADR-0023). Defaults to the APIARY_API_KEY environment variable. Leave unset if the target managerd has no API keys configured yet - every call is unauthenticated in that case.",
			},
		},
	}
}

func (p *ApiaryProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data apiaryProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := data.Endpoint.ValueString()
	if endpoint == "" {
		endpoint = os.Getenv("APIARY_ENDPOINT")
	}
	if endpoint == "" {
		resp.Diagnostics.AddError(
			"Missing Apiary endpoint",
			"The provider requires an endpoint - set it in the provider block's endpoint attribute or the APIARY_ENDPOINT environment variable.",
		)
		return
	}

	apiKey := data.APIKey.ValueString()
	if apiKey == "" {
		apiKey = os.Getenv("APIARY_API_KEY")
	}

	client := NewClient(endpoint, apiKey, &http.Client{})
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *ApiaryProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewVMResource,
		NewJailResource,
		NewNetworkResource,
	}
}

func (p *ApiaryProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
