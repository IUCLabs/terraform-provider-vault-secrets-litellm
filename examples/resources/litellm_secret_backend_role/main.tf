resource "litellm_secret_backend_role" "team_a" {
  backend = litellm_secret_backend.litellm.path
  name    = "team-a"

  models     = ["gpt-3.5-turbo", "gpt-4"]
  max_budget = 50
  ttl        = 3600  # seconds (1h)
  max_ttl    = 86400 # seconds (24h)

  metadata = {
    team = "a"
    env  = "prod"
  }
}
