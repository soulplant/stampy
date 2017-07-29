package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"text/template"
	"time"

	"github.com/hashicorp/vault/api"
)

type Vault struct {
	cmd *exec.Cmd
}

const CONFIG_FILE = "config.dev.json.tmpl"
const PORT = 8001

var ADDRESS = fmt.Sprintf("localhost:%d", PORT)

type args struct {
	Port int
}

func NewVault() *Vault {
	fmt.Println("temp dir", os.TempDir())
	tmpl := template.Must(template.New("config").ParseFiles(CONFIG_FILE))
	f, err := ioutil.TempFile("", "vault-config")
	if err != nil {
		log.Fatal("Failed to open temp file")
	}
	err = tmpl.ExecuteTemplate(f, CONFIG_FILE, &args{
		Port: PORT,
	})
	if err != nil {
		log.Fatal("Failed to execute the script:", err)
	}
	return &Vault{
		cmd: exec.Command("vault", "server", fmt.Sprintf("-config=%s", f.Name())),
	}
}

func (v *Vault) Start() {
	err := v.cmd.Start()
	if err != nil {
		panic("Couldn't start vault")
	}
	success := false
	for i := 0; i < 5; i++ {
		conn, err := net.Dial("tcp", ADDRESS)
		if err != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if conn.Close() != nil {
			log.Fatal("Failed to close the connection")
		}
		success = true
		break
	}
	if !success {
		log.Fatal("Vault failed to start")
	}
}

func (v *Vault) Stop() {
	v.cmd.Process.Kill()
}

func (v *Vault) Client() *api.Client {
	client, err := api.NewClient(&api.Config{
		Address: fmt.Sprintf("http://localhost:%d", PORT),
	})
	if err != nil {
		log.Fatal("Failed to create client", err)
	}
	return client
}
