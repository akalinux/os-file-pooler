#!/bin/bash
gocyclo -over 1 -ignore '_test.*\.go$' .
go test -coverprofile=coverage.out -v
go tool cover -html=coverage.out -o ./cover.html

