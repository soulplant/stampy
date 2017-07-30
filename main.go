package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"text/template"

	"github.com/bgentry/speakeasy"
	"github.com/hashicorp/vault/api"
	"github.com/urfave/cli"
)

var vaultTokenPath = fmt.Sprintf("%s/.vault-token", os.Getenv("HOME"))
var user = os.Getenv("USER")

// readCurrentTokenIfPresent reads a users vault access token from ~/.vault-token.
func readCurrentTokenIfPresent() string {
	token, err := ioutil.ReadFile(vaultTokenPath)
	if err != nil {
		return ""
	}
	return string(token)
}

// loginPrompt attempts to authenticate the user with userpass auth by prompting them
// for their password.
func loginPrompt(vault *api.Client) {
	password, err := speakeasy.Ask("Enter your vault password: ")
	if err != nil {
		panic(err)
	}
	path := fmt.Sprintf("auth/userpass/login/%s", os.Getenv("USER"))
	s, err := vault.Logical().Write(path, map[string]interface{}{
		"password": password,
	})
	if err != nil {
		fmt.Println("Invalid username or password")
		os.Exit(1)
	}

	token := s.Auth.ClientToken
	ioutil.WriteFile(vaultTokenPath, []byte(token), 0600)
	vault.SetToken(token)
}

// setNewPassword prompts the user for a new password to use.
func setNewPassword(vault *api.Client) {
	password, _ := speakeasy.Ask("Enter new password: ")
	passwordConfirm, _ := speakeasy.Ask("Confirm new password: ")
	if password != passwordConfirm {
		fmt.Println("Passwords don't match")
		os.Exit(1)
	}
	path := fmt.Sprintf("auth/userpass/users/%s", user)
	vault.Logical().Write(path, map[string]interface{}{
		"password": password,
	})
}

// readSecret reads the 'value' field of the secret at the given path.
func readSecret(vault *api.Client, path string) (string, error) {
	s, err := vault.Logical().Read(path)
	if err != nil {
		panic(err)
	}
	if s == nil || s.Data == nil {
		return "", errors.New("Couldn't read secret")
	}
	return s.Data["value"].(string), nil
}

// writeSecret writes the 'value' field of the secret at the given path.
func writeSecret(vault *api.Client, path string, value string) error {
	_, err := vault.Logical().Write(path, map[string]interface{}{
		"value": value,
	})
	return err
}

// loginIfNecessary prompts the user for a password to log them in if necessary.
func loginIfNecessary(vault *api.Client) {
	token := readCurrentTokenIfPresent()
	vault.SetToken(token)
	if vault.Token() == "" {
		loginPrompt(vault)
	}
}

// executeTemplate executes the template in the file tmplName, interpolating secrets into it.
func executeTemplate(vault *api.Client, tmplName string) (string, error) {
	secrets := make(map[string]string)
	fails := make(map[string]error)
	discoverSecrets := map[string]interface{}{
		"secret": func(path string) string {
			secrets[path] = ""
			return ""
		},
	}
	tmplBytes, err := ioutil.ReadFile(tmplName)
	if err != nil {
		return "", err
	}

	tmpl := template.New("")
	// Execute the template to discover the secrets it references.
	tmpl.Funcs(discoverSecrets)
	if _, err := tmpl.Parse(string(tmplBytes)); err != nil {
		return "", err
	}
	if err := tmpl.Execute(ioutil.Discard, nil); err != nil {
		return "", err
	}

	failed := false
	for k, _ := range secrets {
		val, err := readSecret(vault, "secret/"+k)
		if err != nil {
			fails[k] = err
			failed = true
			continue
		}
		secrets[k] = val
	}
	if failed {
		var buf bytes.Buffer
		fmt.Fprintln(&buf, "Failed to read all secrets:")
		for k, _ := range fails {
			fmt.Fprintf(&buf, "  Failed to read secret '%v'\n", k)
		}
		return "", errors.New(buf.String())
	}

	lookupSecrets := map[string]interface{}{
		"secret": func(path string) string {
			return secrets[path]
		},
	}
	var buf bytes.Buffer
	tmpl.Funcs(lookupSecrets)
	tmpl.Parse(string(tmplBytes))
	tmpl.Execute(&buf, nil)
	return buf.String(), nil
}

func main() {
	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		vaultAddr = "https://127.0.0.1:8200"
	}
	vault, err := api.NewClient(&api.Config{
		Address: vaultAddr,
	})
	if err != nil {
		fmt.Println("Failed to connect to vault", err)
	}
	app := cli.NewApp()
	app.Name = "vaultage"
	app.Usage = "a simple Vault CLI"
	app.UsageText = "vaultage command [args...]"
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		{
			Name: "login",
			Action: func(c *cli.Context) error {
				loginPrompt(vault)
				return nil
			},
		},
		{
			Name:      "write",
			Usage:     "write or update a secret",
			ArgsUsage: "path",
			Action: func(c *cli.Context) error {
				path := c.Args().Get(0)
				value := c.Args().Get(1)
				if path == "" {
					fmt.Println("usage: vaultage write path [value]")
					os.Exit(1)
				}
				if value == "" {
					value, err = speakeasy.Ask("Enter the secret (it will be hidden): ")
					if err != nil {
						return err
					}
				}
				loginIfNecessary(vault)
				err = writeSecret(vault, path, value)
				if err != nil {
					return err
				}
				return err
			},
		},
		{
			Name:      "set-password",
			Usage:     "set your password",
			ArgsUsage: " ",
			Action: func(c *cli.Context) error {
				loginIfNecessary(vault)
				setNewPassword(vault)
				return nil
			},
		},
		{
			Name:      "stamp",
			Usage:     "write golang templates with Vault secrets interpolated",
			ArgsUsage: "config.json.tmpl > config.tmpl",
			Action: func(c *cli.Context) error {
				loginIfNecessary(vault)

				tmplName := c.Args().First()
				text, err := executeTemplate(vault, tmplName)
				if err != nil {
					fmt.Println("Failed to execute template", err)
					return err
				}
				fmt.Print(text)
				return nil
			},
		},
	}
	app.Run(os.Args)
}
