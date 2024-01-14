export SHELL:=/usr/bin/env bash -O extglob -c
export GO111MODULE:=on
export OS=$(shell uname | tr '[:upper:]' '[:lower:]')

build-wasm: clean
	GOOS=js GOARCH=wasm go build -o 4gewinnt.wasm github.com/fgeller/4gewinnt

test: clean
	go test -v -vet=all -failfast -race

clean:
	rm -f 4gewinnt.wasm

run: 
	go run main.go
