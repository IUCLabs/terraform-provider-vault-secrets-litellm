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
