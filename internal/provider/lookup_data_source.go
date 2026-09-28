package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fEdeko-dev/terraform-provider-vmpanel/internal/client"
)

// lookupDataSource — справочник /vm/v3 по названию: vmpanel_os, vmpanel_location, vmpanel_preset.
type lookupDataSource struct {
	name   string
	path   string
	desc   string
	client *client.Client
}

type lookupModel struct {
	Name    types.String `tfsdk:"name"`
	ID      types.Int64  `tfsdk:"id"`
	State   types.String `tfsdk:"state"`
	CPU     types.Int64  `tfsdk:"cpu"`
	RAMMiB  types.Int64  `tfsdk:"ram_mib"`
	DiskMiB types.Int64  `tfsdk:"disk_mib"`
}

func newLookupDataSource(name, path, desc string) func() datasource.DataSource {
	return func() datasource.DataSource { return &lookupDataSource{name: name, path: path, desc: desc} }
}

func (d *lookupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.name
}

func (d *lookupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: d.desc,
		Attributes: map[string]schema.Attribute{
			"name":     schema.StringAttribute{Required: true, Description: "Название, как в панели (без учёта регистра)."},
			"id":       schema.Int64Attribute{Computed: true, Description: "Номер для /vm/v3 и ресурсов провайдера."},
			"state":    schema.StringAttribute{Computed: true, Description: "У систем: available — можно ставить."},
			"cpu":      schema.Int64Attribute{Computed: true, Description: "У тарифов: ядра."},
			"ram_mib":  schema.Int64Attribute{Computed: true, Description: "У тарифов: память, МиБ."},
			"disk_mib": schema.Int64Attribute{Computed: true, Description: "У тарифов: диск, МиБ."},
		},
	}
}

func (d *lookupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*client.Client); ok {
		d.client = c
	}
}

func (d *lookupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m lookupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	list, err := d.client.List(ctx, d.path)
	if err != nil {
		resp.Diagnostics.AddError("Справочник недоступен", err.Error())
		return
	}
	want := strings.ToLower(strings.TrimSpace(m.Name.ValueString()))
	var names []string
	for _, x := range list {
		names = append(names, x.Name)
		if strings.ToLower(x.Name) == want {
			m.ID = types.Int64Value(x.ID)
			m.State = types.StringValue(x.State)
			m.CPU = types.Int64Value(x.CPU)
			m.RAMMiB = types.Int64Value(x.RAMMiB)
			m.DiskMiB = types.Int64Value(x.HddMiB)
			resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			return
		}
	}
	resp.Diagnostics.AddError(
		fmt.Sprintf("Не найдено: %q", m.Name.ValueString()),
		fmt.Sprintf("Есть: %s", strings.Join(names, ", ")),
	)
}
