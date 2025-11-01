# Go Makefile for git-sweep-go

APP_NAME=git-sweep
CMD_DIR=cmd/git-sweep

.PHONY: all build test lint ci

all: build

build:
	go build -o $(APP_NAME) ./$(CMD_DIR)/main.go

test:
	go test ./... -v

lint:
	golangci-lint run

ci: build lint test
