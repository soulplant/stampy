#!/bin/bash

vault server -config=config.dev.json &> /dev/null &
VAULT_PID=$!

go run main.go

kill $VAULT_PID
