package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// roleData builds the request payload for writing a role to the backend.
func roleData(ctx context.Context, m secretBackendRoleModel) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	data := map[string]interface{}{}

	if !m.Models.IsNull() && !m.Models.IsUnknown() {
		var models []string
		diags.Append(m.Models.ElementsAs(ctx, &models, false)...)
		if diags.HasError() {
			return nil, diags
		}
		data["models"] = models
	} else {
		data["models"] = []string{}
	}

	if !m.MaxBudget.IsNull() && !m.MaxBudget.IsUnknown() {
		data["max_budget"] = m.MaxBudget.ValueFloat64()
	}
	if !m.KeyAliasPrefix.IsNull() && !m.KeyAliasPrefix.IsUnknown() {
		data["key_alias_prefix"] = m.KeyAliasPrefix.ValueString()
	}
	if !m.TTL.IsNull() && !m.TTL.IsUnknown() {
		data["ttl"] = m.TTL.ValueInt64()
	}
	if !m.MaxTTL.IsNull() && !m.MaxTTL.IsUnknown() {
		data["max_ttl"] = m.MaxTTL.ValueInt64()
	}
	if !m.Metadata.IsNull() && !m.Metadata.IsUnknown() {
		var meta map[string]string
		diags.Append(m.Metadata.ElementsAs(ctx, &meta, false)...)
		if diags.HasError() {
			return nil, diags
		}
		data["metadata"] = meta
	}

	return data, diags
}

// applyRoleData refreshes a model from a role read out of the backend.
func applyRoleData(ctx context.Context, m *secretBackendRoleModel, role map[string]interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	models := toStringSlice(role["models"])
	if len(models) == 0 {
		m.Models = types.SetNull(types.StringType)
	} else {
		set, d := types.SetValueFrom(ctx, types.StringType, models)
		diags.Append(d...)
		m.Models = set
	}

	if budget, ok := toFloat64(role["max_budget"]); ok && budget != 0 {
		m.MaxBudget = types.Float64Value(budget)
	} else if m.MaxBudget.IsUnknown() {
		m.MaxBudget = types.Float64Null()
	}

	if prefix, ok := role["key_alias_prefix"].(string); ok {
		m.KeyAliasPrefix = types.StringValue(prefix)
	}

	if ttl, ok := toInt64(role["ttl"]); ok && ttl != 0 {
		m.TTL = types.Int64Value(ttl)
	} else if m.TTL.IsUnknown() {
		m.TTL = types.Int64Null()
	}
	if maxTTL, ok := toInt64(role["max_ttl"]); ok && maxTTL != 0 {
		m.MaxTTL = types.Int64Value(maxTTL)
	} else if m.MaxTTL.IsUnknown() {
		m.MaxTTL = types.Int64Null()
	}

	meta := toStringMap(role["metadata"])
	if len(meta) == 0 {
		m.Metadata = types.MapNull(types.StringType)
	} else {
		mp, d := types.MapValueFrom(ctx, types.StringType, meta)
		diags.Append(d...)
		m.Metadata = mp
	}

	return diags
}

func toStringSlice(v interface{}) []string {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func toStringMap(v interface{}) map[string]string {
	raw, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, e := range raw {
		if s, ok := e.(string); ok {
			out[k] = s
		}
	}
	return out
}

// toInt64 coerces the numeric shapes Vault returns (json.Number, float64, int)
// into an int64.
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			f, ferr := n.Float64()
			if ferr != nil {
				return 0, false
			}
			return int64(f), true
		}
		return i, true
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}

// ensure attr is referenced (kept for future typed conversions).
var _ = attr.Value(nil)
