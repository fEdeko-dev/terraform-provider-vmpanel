package provider

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fEdeko-dev/terraform-provider-vmpanel/internal/client"
)

type webhookResource struct{ client *client.Client }

type webhookModel struct {
	ID          types.String `tfsdk:"id"`
	URL         types.String `tfsdk:"url"`
	Events      types.List   `tfsdk:"events"`
	Description types.String `tfsdk:"description"`
	Active      types.Bool   `tfsdk:"active"`
	Secret      types.String `tfsdk:"secret"`
}

type webhookDTO struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	Description string   `json:"description"`
	Active      bool     `json:"active"`
	Secret      string   `json:"secret,omitempty"`
}

func NewWebhookResource() resource.Resource { return &webhookResource{} }

func (r *webhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *webhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Вебхук: панель шлёт события POST-запросом с подписью X-Webhook-Signature. " +
			"У ключа хостера — события его машин и площадки, у ключа владельца — события всех хостеров и платформы.",
		Attributes: map[string]schema.Attribute{
			"id":  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"url": schema.StringAttribute{Required: true, Description: "Адрес получателя: только https и только внешний."},
			"events": schema.ListAttribute{
				Required: true, ElementType: types.StringType,
				Description: `События: vm.ready, vm.build_failed, vm.deleted, vm.mail_blocked, health.issue_opened, health.issue_resolved; ["*"] — все.`,
			},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"active":      schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"secret": schema.StringAttribute{
				Computed: true, Sensitive: true,
				Description:   "Ключ подписи — проверяйте им X-Webhook-Signature. Панель показывает его только при создании.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *webhookResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*client.Client); ok {
		r.client = c
	}
}

func (r *webhookResource) body(ctx context.Context, m *webhookModel) (map[string]any, bool) {
	var events []string
	if diags := m.Events.ElementsAs(ctx, &events, false); diags.HasError() {
		return nil, false
	}
	return map[string]any{
		"url":         m.URL.ValueString(),
		"events":      events,
		"description": m.Description.ValueString(),
	}, true
}

func (r *webhookResource) apply(ctx context.Context, d *webhookDTO, m *webhookModel) {
	m.ID = types.StringValue(d.ID)
	m.URL = types.StringValue(d.URL)
	m.Description = types.StringValue(d.Description)
	m.Active = types.BoolValue(d.Active)
	if list, diags := types.ListValueFrom(ctx, types.StringType, d.Events); !diags.HasError() {
		m.Events = list
	}
	if d.Secret != "" {
		m.Secret = types.StringValue(d.Secret)
	}
}

func (r *webhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	body, ok := r.body(ctx, &plan)
	if resp.Diagnostics.HasError() || !ok {
		return
	}
	var d webhookDTO
	if err := r.client.Do(ctx, http.MethodPost, "/api/webhooks", body, &d); err != nil {
		resp.Diagnostics.AddError("Вебхук не создан", err.Error())
		return
	}
	if !plan.Active.ValueBool() {
		_ = r.client.Do(ctx, http.MethodPatch, "/api/webhooks/"+d.ID, map[string]any{"active": false}, nil)
		d.Active = false
	}
	r.apply(ctx, &d, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var list []webhookDTO
	if err := r.client.Do(ctx, http.MethodGet, "/api/webhooks", nil, &list); err != nil {
		resp.Diagnostics.AddError("Вебхуки не прочитаны", err.Error())
		return
	}
	for i := range list {
		if list[i].ID == state.ID.ValueString() {
			r.apply(ctx, &list[i], &state)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *webhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state webhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	body, ok := r.body(ctx, &plan)
	if resp.Diagnostics.HasError() || !ok {
		return
	}
	body["active"] = plan.Active.ValueBool()
	var d webhookDTO
	if err := r.client.Do(ctx, http.MethodPatch, "/api/webhooks/"+state.ID.ValueString(), body, &d); err != nil {
		resp.Diagnostics.AddError("Вебхук не изменён", err.Error())
		return
	}
	plan.Secret = state.Secret
	r.apply(ctx, &d, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.Do(ctx, http.MethodDelete, "/api/webhooks/"+state.ID.ValueString(), nil, nil)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Вебхук не удалён", err.Error())
	}
}

func (r *webhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
