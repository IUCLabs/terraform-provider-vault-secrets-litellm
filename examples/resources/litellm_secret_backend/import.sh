# Import by mount path. master_key is never read back, so set it in
# configuration; the next apply rewrites it.
terraform import litellm_secret_backend.litellm litellm
