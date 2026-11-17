#!/usr/bin/env bash
#
# End-to-end test for terraform-provider-vault-secrets-litellm.
#
# Builds the sibling litellm plugin and this provider, boots Vault + LiteLLM +
# Postgres in Docker, then runs a real `terraform apply` through the provider to
# mount the engine and create a role, and asserts a working virtual key can be
# generated from it. Finally `terraform destroy` and verify the mount is gone.
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLUGIN_REPO="${PLUGIN_REPO:-${ROOT_DIR}/../vault-plugin-secrets-litellm}"
COMPOSE_FILE="${ROOT_DIR}/test/docker-compose.yml"
COMPOSE="docker compose -f ${COMPOSE_FILE}"
TF="${TF:-terraform}"
E2E_DIR="${ROOT_DIR}/test/e2e"
PLUGIN_BIN="vault-plugin-secrets-litellm"
PROVIDER_BIN="terraform-provider-vault-secrets-litellm"

MASTER_KEY="sk-master-e2e-1234"
LITELLM_ADDR="http://127.0.0.1:4000"
export VAULT_ADDR="http://127.0.0.1:8200"
export VAULT_TOKEN="root"

red()   { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
blue()  { printf '\033[34m==> %s\033[0m\n' "$*"; }
fail()  { red "FAIL: $*"; exit 1; }

cleanup() {
  blue "Cleaning up"
  if [ -d "${E2E_DIR}" ]; then
    (cd "${E2E_DIR}" && TF_CLI_CONFIG_FILE="${ROOT_DIR}/test/dev.tfrc" "${TF}" destroy -auto-approve >/dev/null 2>&1 || true)
    rm -f "${E2E_DIR}"/terraform.tfstate* "${E2E_DIR}"/.terraform.lock.hcl
    rm -rf "${E2E_DIR}/.terraform"
  fi
  ${COMPOSE} down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

wait_for() {
  local desc="$1"; shift
  local retries="${WAIT_RETRIES:-90}" i=0
  until "$@" >/dev/null 2>&1; do
    i=$((i + 1))
    [ "$i" -ge "$retries" ] && fail "timed out waiting for ${desc}"
    sleep 2
  done
  green "ready: ${desc}"
}

chat_code() {
  curl -s -o /dev/null -w '%{http_code}' \
    -H "Authorization: Bearer $1" -H 'Content-Type: application/json' \
    -d "{\"model\":\"$2\",\"messages\":[{\"role\":\"user\",\"content\":\"hi\"}]}" \
    "${LITELLM_ADDR}/chat/completions"
}

# ---------------------------------------------------------------------------
blue "Building litellm plugin from ${PLUGIN_REPO}"
[ -d "${PLUGIN_REPO}" ] || fail "plugin repo not found at ${PLUGIN_REPO} (set PLUGIN_REPO)"
mkdir -p "${ROOT_DIR}/test/plugins"
( cd "${PLUGIN_REPO}" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" \
    -o "${ROOT_DIR}/test/plugins/${PLUGIN_BIN}" ./cmd/vault-plugin-secrets-litellm )

blue "Building the provider"
( cd "${ROOT_DIR}" && go build -o "${PROVIDER_BIN}" . )

blue "Writing terraform dev_overrides config"
cat > "${ROOT_DIR}/test/dev.tfrc" <<EOF
provider_installation {
  dev_overrides {
    "registry.terraform.io/iuclabs/vault-secrets-litellm" = "${ROOT_DIR}"
  }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE="${ROOT_DIR}/test/dev.tfrc"

blue "Starting Docker stack"
${COMPOSE} up -d --build
wait_for "litellm" curl -fsS "${LITELLM_ADDR}/health/liveliness"
wait_for "vault"   ${COMPOSE} exec -T vault vault status -address=http://127.0.0.1:8200

blue "Registering the litellm plugin in Vault"
SHA="$(sha256sum "${ROOT_DIR}/test/plugins/${PLUGIN_BIN}" | awk '{print $1}')"
${COMPOSE} exec -T vault vault plugin register -sha256="${SHA}" secret "${PLUGIN_BIN}" >/dev/null

# ---------------------------------------------------------------------------
blue "terraform apply (mount engine + create role via the provider)"
( cd "${E2E_DIR}" && "${TF}" apply -auto-approve )
green "apply succeeded"

blue "Verifying the mount and role exist"
${COMPOSE} exec -T vault vault secrets list 2>/dev/null | grep -q '^litellm/' \
  || fail "litellm mount not found after apply"
${COMPOSE} exec -T vault vault read litellm/roles/team-a >/dev/null \
  || fail "role team-a not found after apply"
green "mount + role present"

blue "Generating a virtual key from the terraform-managed role"
KEY="$(${COMPOSE} exec -T vault vault read -field=key litellm/creds/team-a)"
[ -n "${KEY}" ] || fail "no key generated"
green "issued key ${KEY:0:12}..."

code="$(chat_code "${KEY}" gpt-3.5-turbo)"
[ "${code}" = "200" ] || fail "allowed model returned HTTP ${code}, expected 200"
green "allowed model (gpt-3.5-turbo) accepted"

code="$(chat_code "${KEY}" gpt-4)"
[ "${code}" != "200" ] || fail "disallowed model unexpectedly succeeded"
green "disallowed model (gpt-4) rejected (HTTP ${code})"

# ---------------------------------------------------------------------------
blue "terraform destroy (unmount engine)"
( cd "${E2E_DIR}" && "${TF}" destroy -auto-approve )
${COMPOSE} exec -T vault vault secrets list 2>/dev/null | grep -q '^litellm/' \
  && fail "litellm mount still present after destroy" || true
green "mount removed by destroy"

green "ALL PROVIDER END-TO-END CHECKS PASSED"
