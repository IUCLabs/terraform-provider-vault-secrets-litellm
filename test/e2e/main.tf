terraform {
  required_providers {
    litellmvaultsecret = {
      source = "git.unkin.net/unkin/litellmvaultsecret"
    }
  }
}

provider "litellmvaultsecret" {
  address = "http://127.0.0.1:8200"
  token   = "root"
}

resource "litellmvaultsecret_secret_backend" "litellm" {
  path        = "litellm"
  description = "LiteLLM dynamic virtual keys (e2e)"
  # Reachable from inside the vault container, where the plugin runs.
  base_url   = "http://litellm:4000"
  master_key = "sk-master-e2e-1234"
}

resource "litellmvaultsecret_secret_backend_role" "team_a" {
  backend    = litellmvaultsecret_secret_backend.litellm.path
  name       = "team-a"
  models     = ["gpt-3.5-turbo"]
  max_budget = 10
  ttl        = 3600
  max_ttl    = 86400
}
