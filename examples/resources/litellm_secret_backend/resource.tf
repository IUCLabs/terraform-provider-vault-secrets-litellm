resource "litellm_secret_backend" "litellm" {
  path        = "litellm"
  description = "LiteLLM virtual keys"
  base_url    = "https://litellm.example.com"
  master_key  = var.litellm_master_key
}

variable "litellm_master_key" {
  type      = string
  sensitive = true
}
