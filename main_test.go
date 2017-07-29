package main

import (
	"bytes"
	"testing"
	"text/template"
)

func Test_main(t *testing.T) {
	t.Run("foo", func(t *testing.T) {
		vault := NewVault()
		vault.Start()
		defer vault.Stop()
		client := vault.Client()
		b, err := client.Sys().InitStatus()
		if err != nil {
			t.Fatal("Couldn't get the status", err)
		}
		if b {
			t.Fatal("Shouldn't be inited")
		}
	})
}

type foo struct {
	Foo func(n int) string
}

func Test_foo(t *testing.T) {
	var buf bytes.Buffer
	fm := map[string]interface{}{
		"Foo": func(n int) string {
			if n == 3 {
				return "hello"
			}
			return "bye"
		},
	}

	tmpl := template.New("mine")
	tmpl.Parse("{{call .Foo 3}}")
	tmpl.Funcs(fm)

	f := foo{
		Foo: func(n int) string {
			if n == 3 {
				return "hello"
			}
			return "hi"
		},
	}
	err := tmpl.Execute(&buf, f)
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "hello" {
		t.Fatal("expected hello, got", buf.String())
	}
}
