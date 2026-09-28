// Package provider — провайдер Terraform для панели управления виртуальными машинами.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fEdeko-dev/terraform-provider-vmpanel/internal/client"
)

type panelProvider struct {
	version string
}

type providerModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &panelProvider{version: version} }
}

func (p *panelProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "vmpanel"
	resp.Version = p.version
}

func (p *panelProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Машины и вебхуки панели. Вход — ключом API: у клиента хостера ключ видит только его машины, " +
			"у сотрудника — парк организации, у владельца — платформу.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:    true,
				Description: "Адрес панели, например https://panel.example.ru. Или переменная VMPANEL_ENDPOINT.",
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Ключ API (vtk_…): «Настройки» → «Ключи API». Или переменная VMPANEL_API_KEY.",
			},
		},
	}
}

func (p *panelProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpoint := os.Getenv("VMPANEL_ENDPOINT")
	if !cfg.Endpoint.IsNull() && !cfg.Endpoint.IsUnknown() {
		endpoint = cfg.Endpoint.ValueString()
	}
	key := os.Getenv("VMPANEL_API_KEY")
	if !cfg.APIKey.IsNull() && !cfg.APIKey.IsUnknown() {
		key = cfg.APIKey.ValueString()
	}
	if endpoint == "" {
		resp.Diagnostics.AddError("Не задан адрес панели", "Укажите endpoint в блоке provider или переменную VMPANEL_ENDPOINT.")
	}
	if key == "" {
		resp.Diagnostics.AddError("Не задан ключ API", "Укажите api_key в блоке provider или переменную VMPANEL_API_KEY.")
	}
	if resp.Diagnostics.HasError() {
		return
	}
	c := client.New(endpoint, key)
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *panelProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewVMResource, NewWebhookResource}
}

func (p *panelProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newLookupDataSource("os", "os", "Система из каталога по названию, например «Ubuntu 24.04»."),
		newLookupDataSource("location", "cluster", "Локация по названию."),
		newLookupDataSource("preset", "preset", "Тариф (пресет) по названию."),
	}
}
