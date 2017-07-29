package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
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
func readSecret(vault *api.Client, path string) string {
	s, err := vault.Logical().Read(path)
	if err != nil {
		panic(err)
	}
	return s.Data["value"].(string)
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

func main() {
	var loginFlag bool
	var writeFlag bool
	var passwordFlag bool
	var address string
	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		vaultAddr = "https://127.0.0.1:8200"
	}
	app := cli.NewApp()
	app.Usage = "stamps out golang templates with vault secrets in them"
	app.UsageText = "stampy config.tmpl"
	app.Version = "0.0.1"
	app.ArgsUsage = "config.tmpl"
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "login, l",
			Usage:       "Just login and exit",
			Destination: &loginFlag,
		},
		cli.StringFlag{
			Name:        "address, a",
			Usage:       "The address of the vault server",
			Destination: &address,
			Value:       vaultAddr,
		},
		cli.BoolFlag{
			Name:        "write, w",
			Usage:       "Write the value of a secret",
			Destination: &writeFlag,
		},
		cli.BoolFlag{
			Name:        "pw",
			Usage:       "Change your password",
			Destination: &passwordFlag,
		},
	}
	app.Action = func(c *cli.Context) error {
		vault, err := api.NewClient(&api.Config{
			Address: address,
		})
		if err != nil {
			fmt.Println("Failed to connect to vault", err)
		}
		if loginFlag {
			loginPrompt(vault)
			os.Exit(0)
		}
		if writeFlag {
			path := c.Args().First()
			value := c.Args().Get(1)
			if path == "" {
				fmt.Println("usage: stampy -w path [value]")
				os.Exit(1)
			}
			if value == "" {
				value, err = speakeasy.Ask("Enter the secret (it will be hidden): ")
				if err != nil {
					panic(err)
				}
			}
			loginIfNecessary(vault)
			_, err := vault.Logical().Write(path, map[string]interface{}{
				"value": value,
			})
			if err != nil {
				log.Println("Failed to write secret:", err)
			}
			os.Exit(0)
		}
		if passwordFlag {
			loginIfNecessary(vault)
			setNewPassword(vault)
			os.Exit(0)
		}
		if c.Args().First() == "" {
			fmt.Println("usage:", app.UsageText)
			os.Exit(1)
		}
		loginIfNecessary(vault)

		tmpl := template.New("").Funcs(
			map[string]interface{}{
				"secret": func(path string) string { return readSecret(vault, "secret/"+path) },
			})
		tmplName := c.Args().First()
		t, err := tmpl.ParseFiles(tmplName)
		if err != nil {
			log.Fatal(err)
		}
		var buf bytes.Buffer
		t.ExecuteTemplate(&buf, tmplName, nil)
		fmt.Print(buf.String())
		return nil
	}
	app.Run(os.Args)
}
