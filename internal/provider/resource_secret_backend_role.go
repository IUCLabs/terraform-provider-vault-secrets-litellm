package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &secretBackendRoleResource{}
	_ resource.ResourceWithImportState = &secretBackendRoleResource{}
)

type secretBackendRoleResource struct {
	client *vaultClient
}

type secretBackendRoleModel struct {
	Backend        types.String  `tfsdk:"backend"`
	Name           types.String  `tfsdk:"name"`
	Models         types.Set     `tfsdk:"models"`
	MaxBudget      types.Float64 `tfsdk:"max_budget"`
	KeyAliasPrefix types.String  `tfsdk:"key_alias_prefix"`
	TTL            types.Int64   `tfsdk:"ttl"`
	MaxTTL         types.Int64   `tfsdk:"max_ttl"`
	Metadata       types.Map     `tfsdk:"metadata"`
}

func NewSecretBackendRoleResource() resource.Resource {
	return &secretBackendRoleResource{}
}

func (r *secretBackendRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret_backend_role"
}

func (r *secretBackendRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a role on the LiteLLM secrets engine that constrains generated virtual keys.",
		Attributes: map[string]schema.Attribute{
			"backend": schema.StringAttribute{
				Description: "Mount path of the LiteLLM secrets engine. Changing this forces a new resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the role. Keys are generated from creds/<name>. Changing this forces a new resource.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"models": schema.SetAttribute{
				Description: "Models a generated key may access. Empty means unrestricted.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"max_budget": schema.Float64Attribute{
				Description: "Spending limit applied to each generated key. 0 means unlimited.",
				Optional:    true,
			},
			"key_alias_prefix": schema.StringAttribute{
				Description: "Prefix for the auto-generated key alias. Defaults to \"vault\".",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("vault"),
			},
			"ttl": schema.Int64Attribute{
				Description: "Default lease TTL in seconds for keys generated from this role.",
				Optional:    true,
			},
			"max_ttl": schema.Int64Attribute{
				Description: "Maximum lease TTL in seconds for keys generated from this role.",
				Optional:    true,
			},
			"metadata": schema.MapAttribute{
				Description: "Metadata attached to each generated key.",
				ElementType: types.StringType,
				Optional:    true,
			},
		},
	}
}

func (r *secretBackendRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*vaultClient)
	if !ok {
		resp.Diagnostics.AddError("unexpected provider data type", fmt.Sprintf("got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *secretBackendRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan secretBackendRoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data, diags := roleData(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.writeRole(ctx, plan.Backend.ValueString(), plan.Name.ValueString(), data); err != nil {
		resp.Diagnostics.AddError("failed to create litellm role", err.Error())
		return
	}
	resp.Diagnostics.Append(r.readInto(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *secretBackendRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state secretBackendRoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.readRole(ctx, state.Backend.ValueString(), state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to read litellm role", err.Error())
		return
	}
	if role == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	diags := applyRoleData(ctx, &state, role)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *secretBackendRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan secretBackendRoleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data, diags := roleData(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.writeRole(ctx, plan.Backend.ValueString(), plan.Name.ValueString(), data); err != nil {
		resp.Diagnostics.AddError("failed to update litellm role", err.Error())
		return
	}
	resp.Diagnostics.Append(r.readInto(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *secretBackendRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state secretBackendRoleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.deleteRole(ctx, state.Backend.ValueString(), state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete litellm role", err.Error())
		return
	}
}

func (r *secretBackendRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import ID format: "<backend>/roles/<name>".
	backend, name, ok := splitRoleID(req.ID)
	if !ok {
		resp.Diagnostics.AddError(
			"invalid import ID",
			fmt.Sprintf("expected \"<backend>/roles/<name>\", got %q", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("backend"), backend)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

// readInto refreshes the model in place from the backend after a write, so
// computed fields (e.g. key_alias_prefix default) reflect stored values.
func (r *secretBackendRoleResource) readInto(ctx context.Context, m *secretBackendRoleModel) diag.Diagnostics {
	var diags diag.Diagnostics
	role, err := r.client.readRole(ctx, m.Backend.ValueString(), m.Name.ValueString())
	if err != nil {
		diags.AddError("failed to read back litellm role", err.Error())
		return diags
	}
	if role == nil {
		diags.AddError("role missing after write", "the role was not found immediately after being written")
		return diags
	}
	return applyRoleData(ctx, m, role)
}

func splitRoleID(id string) (backend, name string, ok bool) {
	marker := "/roles/"
	idx := strings.LastIndex(id, marker)
	if idx <= 0 {
		return "", "", false
	}
	backend = id[:idx]
	name = id[idx+len(marker):]
	if backend == "" || name == "" {
		return "", "", false
	}
	return backend, name, true
}
