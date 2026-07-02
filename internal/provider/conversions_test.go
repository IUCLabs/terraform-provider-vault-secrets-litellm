package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRoleData_FullRoundTrip(t *testing.T) {
	ctx := context.Background()

	models, _ := types.SetValueFrom(ctx, types.StringType, []string{"gpt-4", "gpt-3.5-turbo"})
	meta, _ := types.MapValueFrom(ctx, types.StringType, map[string]string{"team": "a"})

	m := secretBackendRoleModel{
		Backend:        types.StringValue("litellm"),
		Name:           types.StringValue("team-a"),
		Models:         models,
		MaxBudget:      types.Float64Value(50),
		KeyAliasPrefix: types.StringValue("vault"),
		TTL:            types.Int64Value(3600),
		MaxTTL:         types.Int64Value(86400),
		Metadata:       meta,
	}

	data, diags := roleData(ctx, m)
	if diags.HasError() {
		t.Fatalf("roleData diags: %v", diags)
	}
	if got := data["max_budget"].(float64); got != 50 {
		t.Fatalf("max_budget: %v", got)
	}
	if got := data["ttl"].(int64); got != 3600 {
		t.Fatalf("ttl: %v", got)
	}
	if got := data["models"].([]string); len(got) != 2 {
		t.Fatalf("models: %v", got)
	}
	if got := data["metadata"].(map[string]string); got["team"] != "a" {
		t.Fatalf("metadata: %v", got)
	}
}

func TestRoleData_OmitsUnsetOptionals(t *testing.T) {
	ctx := context.Background()
	m := secretBackendRoleModel{
		Backend:        types.StringValue("litellm"),
		Name:           types.StringValue("minimal"),
		Models:         types.SetNull(types.StringType),
		MaxBudget:      types.Float64Null(),
		KeyAliasPrefix: types.StringValue("vault"),
		TTL:            types.Int64Null(),
		MaxTTL:         types.Int64Null(),
		Metadata:       types.MapNull(types.StringType),
	}
	data, diags := roleData(ctx, m)
	if diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if _, ok := data["max_budget"]; ok {
		t.Fatal("max_budget should be omitted when null")
	}
	if _, ok := data["ttl"]; ok {
		t.Fatal("ttl should be omitted when null")
	}
	// models is always sent (empty list = unrestricted).
	if got, ok := data["models"].([]string); !ok || len(got) != 0 {
		t.Fatalf("expected empty models slice, got %v", data["models"])
	}
}

func TestApplyRoleData(t *testing.T) {
	ctx := context.Background()
	m := &secretBackendRoleModel{
		Models:    types.SetNull(types.StringType),
		MaxBudget: types.Float64Null(),
		TTL:       types.Int64Null(),
		MaxTTL:    types.Int64Null(),
		Metadata:  types.MapNull(types.StringType),
	}

	// Simulate what Vault returns (numbers arrive as json.Number).
	role := map[string]interface{}{
		"models":           []interface{}{"gpt-4"},
		"max_budget":       json.Number("50"),
		"key_alias_prefix": "vault",
		"ttl":              json.Number("3600"),
		"max_ttl":          json.Number("86400"),
		"metadata":         map[string]interface{}{"team": "a"},
	}

	diags := applyRoleData(ctx, m, role)
	if diags.HasError() {
		t.Fatalf("applyRoleData diags: %v", diags)
	}
	if m.TTL.ValueInt64() != 3600 {
		t.Fatalf("ttl: %v", m.TTL)
	}
	if m.MaxTTL.ValueInt64() != 86400 {
		t.Fatalf("max_ttl: %v", m.MaxTTL)
	}
	if m.MaxBudget.ValueFloat64() != 50 {
		t.Fatalf("max_budget: %v", m.MaxBudget)
	}
	if m.KeyAliasPrefix.ValueString() != "vault" {
		t.Fatalf("prefix: %v", m.KeyAliasPrefix)
	}
	var models []string
	m.Models.ElementsAs(ctx, &models, false)
	if len(models) != 1 || models[0] != "gpt-4" {
		t.Fatalf("models: %v", models)
	}
}

func TestApplyRoleData_EmptyModelsBecomesNull(t *testing.T) {
	ctx := context.Background()
	m := &secretBackendRoleModel{
		Models:   types.SetValueMust(types.StringType, nil),
		Metadata: types.MapNull(types.StringType),
	}
	role := map[string]interface{}{"models": []interface{}{}}
	if diags := applyRoleData(ctx, m, role); diags.HasError() {
		t.Fatalf("diags: %v", diags)
	}
	if !m.Models.IsNull() {
		t.Fatalf("expected null models, got %v", m.Models)
	}
}

func TestToInt64(t *testing.T) {
	cases := []struct {
		in   interface{}
		want int64
		ok   bool
	}{
		{json.Number("3600"), 3600, true},
		{float64(42), 42, true},
		{int64(7), 7, true},
		{int(9), 9, true},
		{"nope", 0, false},
		{nil, 0, false},
	}
	for _, tc := range cases {
		got, ok := toInt64(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("toInt64(%v) = (%d,%v), want (%d,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestSplitRoleID(t *testing.T) {
	backend, name, ok := splitRoleID("litellm/roles/team-a")
	if !ok || backend != "litellm" || name != "team-a" {
		t.Fatalf("got backend=%q name=%q ok=%v", backend, name, ok)
	}
	if _, _, ok := splitRoleID("nonsense"); ok {
		t.Fatal("expected failure on malformed id")
	}
	// Nested mount paths must still split on the final /roles/.
	b, n, ok := splitRoleID("team/litellm/roles/x")
	if !ok || b != "team/litellm" || n != "x" {
		t.Fatalf("nested split: backend=%q name=%q ok=%v", b, n, ok)
	}
}
