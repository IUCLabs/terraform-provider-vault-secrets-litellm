terraform {
  required_providers {
    litellmvaultsecret = {
      source = "git.unkin.net/unkin/litellmvaultsecret"
    }
  }
}

provider "litellmvaultsecret" {
  # address and token fall back to VAULT_ADDR / VAULT_TOKEN
  address = "https://vault.example.com"
}

resource "litellmvaultsecret_secret_backend" "litellm" {
  path        = "litellm"
  description = "LiteLLM dynamic virtual keys"
  base_url    = "http://litellm.litellm.svc:4000"
  master_key  = var.litellm_master_key
}

variable "litellm_master_key" {
  type      = string
  sensitive = true
}
