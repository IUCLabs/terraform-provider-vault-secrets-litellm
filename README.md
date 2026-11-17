# Terraform Provider for the LiteLLM Vault Secrets Engine

This provider manages the LiteLLM dynamic secrets engine,
[vault-plugin-secrets-litellm](https://github.com/IUCLabs/vault-plugin-secrets-litellm),
on HashiCorp Vault or OpenBao. It mounts the engine, configures its connection
to a [LiteLLM](https://github.com/BerriAI/litellm) proxy, and manages the roles
that scope the virtual keys clients generate from `creds/<role>`.

## Quick Start

Prebuilt provider archives are on the [releases page](https://github.com/IUCLabs/terraform-provider-vault-secrets-litellm/releases).

The registry type is `vault-secrets-litellm`; its resources are prefixed
`litellm_`, so declare the provider under the local name `litellm`:

```terraform
terraform {
  required_providers {
    litellm = {
      source = "IUCLabs/vault-secrets-litellm"
    }
  }
}

# address and token fall back to VAULT_ADDR and VAULT_TOKEN
provider "litellm" {
  address = "https://vault.example.com:8200"
}

resource "litellm_secret_backend" "litellm" {
  path       = "litellm"
  base_url   = "https://litellm.example.com"
  master_key = var.litellm_master_key
}

resource "litellm_secret_backend_role" "team_a" {
  backend    = litellm_secret_backend.litellm.path
  name       = "team-a"
  models     = ["gpt-4o", "claude-3-5-sonnet"]
  max_budget = 50
  ttl        = 3600
  max_ttl    = 86400
}
```

Clients then run `vault read litellm/creds/team-a` to get a scoped, budgeted,
lease-bound virtual key.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) 1.0+ or OpenTofu
- Vault or OpenBao (tested against Vault 1.18) with
  [vault-plugin-secrets-litellm](https://github.com/IUCLabs/vault-plugin-secrets-litellm)
  registered in the plugin catalog
- [Go](https://go.dev/doc/install) 1.25+ (to build the provider)

## Usage

See the [provider documentation](docs/index.md) and the resource pages:

- [litellm_secret_backend](docs/resources/secret_backend.md)
- [litellm_secret_backend_role](docs/resources/secret_backend_role.md)

The engine's paths and parameters are documented in the
[plugin repository](https://github.com/IUCLabs/vault-plugin-secrets-litellm/tree/main/docs).

## Development

```sh
make build   # build the provider binary
make install # install into ~/.terraform.d/plugins for local use
make test    # unit tests
make docs    # regenerate docs/ from the schema and examples/
make e2e     # end-to-end tests in Docker (needs the plugin repo checked out alongside)
```

## License

[MIT](LICENSE)
