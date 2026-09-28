package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fEdeko-dev/terraform-provider-vmpanel/internal/client"
)

const (
	buildTimeout  = 60 * time.Minute
	changeTimeout = 20 * time.Minute
)

type vmResource struct{ client *client.Client }

type vmModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	OSID       types.Int64  `tfsdk:"os_id"`
	LocationID types.Int64  `tfsdk:"location_id"`
	PresetID   types.Int64  `tfsdk:"preset_id"`
	CPU        types.Int64  `tfsdk:"cpu"`
	RAMMiB     types.Int64  `tfsdk:"ram_mib"`
	DiskMiB    types.Int64  `tfsdk:"disk_mib"`
	Password   types.String `tfsdk:"password"`
	IPv4       types.String `tfsdk:"ipv4"`
	IPv4All    types.List   `tfsdk:"ipv4_addresses"`
	State      types.String `tfsdk:"state"`
	OSName     types.String `tfsdk:"os_name"`
	Node       types.String `tfsdk:"node"`
}

func NewVMResource() resource.Resource { return &vmResource{} }

func (r *vmResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm"
}

func (r *vmResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "Виртуальная машина. Создание ждёт, пока машина станет active: адрес и пароль уже настроены в госте. " +
			"Ядра, память, диск и пароль меняются без пересоздания; имя, система, локация и тариф — пересозданием.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Номер машины в /vm/v3 (host_id).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required: true, Description: "Имя машины: латиница, цифры, «-», «.», до 49 символов.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"os_id": schema.Int64Attribute{
				Required: true, Description: "Система: id из data.vmpanel_os.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"location_id": schema.Int64Attribute{
				Optional: true, Computed: true, Description: "Локация: id из data.vmpanel_location. Не задана — первая доступная.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplaceIfConfigured(), int64planmodifier.UseStateForUnknown()},
			},
			"preset_id": schema.Int64Attribute{
				Optional: true, Description: "Тариф: id из data.vmpanel_preset. Задаёт ядра, память и диск, если они не указаны.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"cpu":      schema.Int64Attribute{Optional: true, Computed: true, Description: "Ядра. Смена — перезапуском машины.", PlanModifiers: keep},
			"ram_mib":  schema.Int64Attribute{Optional: true, Computed: true, Description: "Память, МиБ. Смена — перезапуском машины.", PlanModifiers: keep},
			"disk_mib": schema.Int64Attribute{Optional: true, Computed: true, Description: "Диск, МиБ. Можно только увеличить — раздел растянется в госте.", PlanModifiers: keep},
			"password": schema.StringAttribute{
				Required: true, Sensitive: true,
				Description: "Пароль root (Administrator у Windows): 6–128 печатных символов латиницы без пробела. Смена применяется в работающей машине.",
			},
			"ipv4": schema.StringAttribute{
				Computed: true, Description: "Основной адрес.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ipv4_addresses": schema.ListAttribute{
				Computed: true, ElementType: types.StringType, Description: "Все адреса IPv4 машины.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"state": schema.StringAttribute{Computed: true, Description: "active, stopped, suspended, creating, crashed."},
			"os_name": schema.StringAttribute{
				Computed: true, Description: "Установленная система.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"node": schema.StringAttribute{
				Computed: true, Description: "Нода Proxmox, на которой стоит машина.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *vmResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if c, ok := req.ProviderData.(*client.Client); ok {
		r.client = c
	}
}

// fill переносит машину из API в модель. Пароль и os_id API не отдаёт — их
// берём из конфигурации (os_id после импорта восстанавливается по названию).
func (r *vmResource) fill(ctx context.Context, h *client.Host, m *vmModel) error {
	m.ID = types.StringValue(strconv.FormatInt(h.ID, 10))
	m.Name = types.StringValue(h.Name)
	m.LocationID = types.Int64Value(h.Cluster.ID)
	m.CPU = types.Int64Value(h.CPU)
	m.RAMMiB = types.Int64Value(h.RAMMiB)
	m.DiskMiB = types.Int64Value(h.DiskMiB)
	m.State = types.StringValue(h.State)
	m.OSName = types.StringValue(h.OSName)
	m.Node = types.StringValue(h.Node.Name)
	ips := make([]string, 0, len(h.IP4))
	for _, a := range h.IP4 {
		ips = append(ips, a.IP)
	}
	if len(ips) > 0 {
		m.IPv4 = types.StringValue(ips[0])
	} else {
		m.IPv4 = types.StringValue("")
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, ips)
	if diags.HasError() {
		return fmt.Errorf("адреса: %v", diags)
	}
	m.IPv4All = list
	if m.OSID.IsNull() || m.OSID.IsUnknown() {
		if oss, err := r.client.List(ctx, "os"); err == nil {
			for _, o := range oss {
				if strings.EqualFold(o.Name, h.OSName) {
					m.OSID = types.Int64Value(o.ID)
				}
			}
		}
	}
	return nil
}

func hostID(m *vmModel) (int64, error) {
	return strconv.ParseInt(m.ID.ValueString(), 10, 64)
}

func (r *vmResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vmModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{
		"name":     plan.Name.ValueString(),
		"os":       plan.OSID.ValueInt64(),
		"password": plan.Password.ValueString(),
	}
	setIf := func(key string, v types.Int64) {
		if !v.IsNull() && !v.IsUnknown() {
			body[key] = v.ValueInt64()
		}
	}
	setIf("cluster", plan.LocationID)
	setIf("preset", plan.PresetID)
	setIf("cpu_number", plan.CPU)
	setIf("ram_mib", plan.RAMMiB)
	setIf("hdd_mib", plan.DiskMiB)

	var ref client.TaskRef
	if err := r.client.Do(ctx, http.MethodPost, "/vm/v3/host", body, &ref); err != nil {
		resp.Diagnostics.AddError("Машина не заказана", err.Error())
		return
	}
	// Номер — в состояние сразу: сорвётся сборка — машина останется помеченной
	// на пересоздание, а не потерянной
	plan.ID = types.StringValue(strconv.FormatInt(ref.ID, 10))
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)

	if err := r.client.WaitTask(ctx, ref.Task, buildTimeout); err != nil {
		resp.Diagnostics.AddError("Сборка машины не удалась", err.Error())
		return
	}
	h, err := r.client.WaitActive(ctx, ref.ID, buildTimeout)
	if err != nil {
		resp.Diagnostics.AddError("Машина не стала active", err.Error())
		return
	}
	if err := r.fill(ctx, h, &plan); err != nil {
		resp.Diagnostics.AddError("Ответ панели не разобран", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vmResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vmModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := hostID(&state)
	if err != nil {
		resp.Diagnostics.AddError("Неверный id", err.Error())
		return
	}
	h, err := r.client.GetHost(ctx, id)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Машина не прочитана", err.Error())
		return
	}
	if err := r.fill(ctx, h, &state); err != nil {
		resp.Diagnostics.AddError("Ответ панели не разобран", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vmResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state vmModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := hostID(&state)
	if err != nil {
		resp.Diagnostics.AddError("Неверный id", err.Error())
		return
	}

	// Ядра и память — одним запросом, панель перезапустит машину сама
	res := map[string]any{}
	if !plan.CPU.IsUnknown() && !plan.CPU.Equal(state.CPU) {
		res["cpu_number"] = plan.CPU.ValueInt64()
	}
	if !plan.RAMMiB.IsUnknown() && !plan.RAMMiB.Equal(state.RAMMiB) {
		res["ram_mib"] = plan.RAMMiB.ValueInt64()
	}
	if len(res) > 0 {
		var ref client.TaskRef
		if err := r.client.Do(ctx, http.MethodPost, fmt.Sprintf("/vm/v3/host/%d/resource", id), res, &ref); err != nil {
			resp.Diagnostics.AddError("Ресурсы не изменены", err.Error())
			return
		}
		if err := r.client.WaitTask(ctx, ref.Task, changeTimeout); err != nil {
			resp.Diagnostics.AddError("Ресурсы не изменены", err.Error())
			return
		}
	}

	if !plan.DiskMiB.IsUnknown() && !plan.DiskMiB.Equal(state.DiskMiB) {
		if plan.DiskMiB.ValueInt64() < state.DiskMiB.ValueInt64() {
			resp.Diagnostics.AddError("Диск нельзя уменьшить",
				fmt.Sprintf("Сейчас %d МиБ, в конфигурации %d. Уменьшение — только пересозданием машины.", state.DiskMiB.ValueInt64(), plan.DiskMiB.ValueInt64()))
			return
		}
		var disks struct {
			List []struct {
				ID int64 `json:"id"`
			} `json:"list"`
		}
		where := url.QueryEscape(fmt.Sprintf("host.id EQ %d", id))
		if err := r.client.Do(ctx, http.MethodGet, "/vm/v3/disk?where="+where, nil, &disks); err != nil || len(disks.List) == 0 {
			resp.Diagnostics.AddError("Диск машины не найден", fmt.Sprint(err))
			return
		}
		var ref client.TaskRef
		if err := r.client.Do(ctx, http.MethodPost, fmt.Sprintf("/vm/v3/disk/%d", disks.List[0].ID),
			map[string]any{"size_mib": plan.DiskMiB.ValueInt64()}, &ref); err != nil {
			resp.Diagnostics.AddError("Диск не увеличен", err.Error())
			return
		}
		if err := r.client.WaitTask(ctx, ref.Task, changeTimeout); err != nil {
			resp.Diagnostics.AddError("Диск не увеличен", err.Error())
			return
		}
	}

	if !plan.Password.Equal(state.Password) {
		if err := r.client.Do(ctx, http.MethodPost, fmt.Sprintf("/vm/v3/host/%d/password", id),
			map[string]any{"password": plan.Password.ValueString()}, nil); err != nil {
			resp.Diagnostics.AddError("Пароль не изменён", err.Error())
			return
		}
	}

	h, err := r.client.WaitActive(ctx, id, changeTimeout)
	if err != nil {
		resp.Diagnostics.AddError("Машина не вернулась в active", err.Error())
		return
	}
	if err := r.fill(ctx, h, &plan); err != nil {
		resp.Diagnostics.AddError("Ответ панели не разобран", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vmResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := hostID(&state)
	if err != nil {
		return
	}
	var ref client.TaskRef
	err = r.client.Do(ctx, http.MethodDelete, fmt.Sprintf("/vm/v3/host/%d?force=true", id), nil, &ref)
	if client.IsNotFound(err) {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Машина не удалена", err.Error())
		return
	}
	if err := r.client.WaitTask(ctx, ref.Task, changeTimeout); err != nil {
		resp.Diagnostics.AddError("Машина не удалена", err.Error())
	}
}

// ImportState: terraform import vmpanel_vm.web 42 — номер машины в /vm/v3.
func (r *vmResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
