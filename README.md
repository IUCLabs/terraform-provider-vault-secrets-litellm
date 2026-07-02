# terraform-provider-litellmvaultsecret

A Terraform/OpenTofu provider that manages the **LiteLLM dynamic secrets engine**
(the [`vault-plugin-secrets-litellm`](https://git.unkin.net/unkin/vault-plugin-secrets-litellm)
plugin) on HashiCorp Vault or OpenBao.

It lets you declare, as code, the LiteLLM secrets-engine mount, its connection
config, and the roles that scope generated virtual keys — for use from
`terraform-vault`.

## Provider

```hcl
terraform {
  required_providers {
    litellmvaultsecret = {
      source = "git.unkin.net/unkin/litellmvaultsecret"
    }
  }
}

provider "litellmvaultsecret" {
  address = "https://vault.example.com" # or VAULT_ADDR
  token   = var.vault_token             # or VAULT_TOKEN
}
```

## Resources

### `litellmvaultsecret_secret_backend`

Mounts the engine and writes its connection config.

| Attribute                 | Required | Description                                          |
| ------------------------- | -------- | ---------------------------------------------------- |
| `path`                    | yes      | Mount path (e.g. `litellm`). Forces replacement.     |
| `base_url`                | yes      | LiteLLM proxy URL the plugin calls.                  |
| `master_key`              | yes      | LiteLLM master key (sensitive, never read back).     |
| `plugin`                  | no       | Registered plugin name (default `vault-plugin-secrets-litellm`). |
| `description`             | no       | Mount description.                                   |
| `request_timeout_seconds` | no       | Plugin→LiteLLM HTTP timeout (default 30).            |

### `litellmvaultsecret_secret_backend_role`

Manages a role that constrains generated keys.

| Attribute          | Required | Description                                    |
| ------------------ | -------- | ---------------------------------------------- |
| `backend`          | yes      | Mount path of the engine. Forces replacement.  |
| `name`             | yes      | Role name. Forces replacement.                 |
| `models`           | no       | Allowed models (set); empty = unrestricted.    |
| `max_budget`       | no       | Spending limit per key; 0 = unlimited.         |
| `ttl`              | no       | Default lease TTL, in **seconds**.             |
| `max_ttl`          | no       | Maximum lease TTL, in **seconds**.             |
| `key_alias_prefix` | no       | Prefix for the key alias (default `vault`).    |
| `metadata`         | no       | Metadata attached to each key (map).           |

## Example

```hcl
resource "litellmvaultsecret_secret_backend" "litellm" {
  path       = "litellm"
  base_url   = "http://litellm.litellm.svc:4000"
  master_key = var.litellm_master_key
}

resource "litellmvaultsecret_secret_backend_role" "team_a" {
  backend    = litellmvaultsecret_secret_backend.litellm.path
  name       = "team-a"
  models     = ["gpt-3.5-turbo", "gpt-4"]
  max_budget = 50
  ttl        = 3600
  max_ttl    = 86400
}
```

Consumers then read `litellm/creds/team-a` from Vault to obtain a scoped,
budgeted, lease-bound virtual key.

## Import

```sh
terraform import litellmvaultsecret_secret_backend.litellm litellm
terraform import litellmvaultsecret_secret_backend_role.team_a litellm/roles/team-a
```

## Development

```sh
make build   # build the provider binary
make install # install into ~/.terraform.d/plugins for local use
make test    # unit tests (race-enabled)
make lint    # go vet
make fmt     # gofmt
make e2e     # end-to-end: real Vault + LiteLLM + plugin, terraform apply
```

### End-to-end tests

`make e2e` builds the sibling `vault-plugin-secrets-litellm` plugin, boots Vault +
LiteLLM + Postgres in Docker, installs this provider locally, then runs a real
`terraform apply` that mounts the engine and creates a role, and asserts that a
working virtual key can be generated from it. Requires Docker; bind mounts use
`:z` for SELinux.
