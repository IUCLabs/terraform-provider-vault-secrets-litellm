resource "litellm_secret_backend_role" "team_a" {
  backend = litellm_secret_backend.litellm.path
  name    = "team-a"

  models     = ["gpt-4o", "claude-3-5-sonnet"]
  max_budget = 50
  ttl        = 3600
  max_ttl    = 86400

  metadata = {
    team = "a"
  }
}
