# Stampy
Stamps out Vault secrets into Golang templates.

Stampy assumes that the current user has a userpass login for Vault with the same name as $USER. After login in it will store its token at ~/.vault-token.

# Installation
`go install github.com/soulplant/stampy`

# Usage
```
stampy config.json.tmpl > config.json  # Stamp out a config file
stampy -write secret/some/secret       # Write a secret
stampy -login                          # Just login, no stamping
stampy -pw                             # Update your password

# Use VAULT_ADDR to configure which vault to talk to.
VAULT_ADDR="https://my-vault.com:8200" stampy foo > bar
```