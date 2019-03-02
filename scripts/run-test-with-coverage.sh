#!/bin/bash

echo "running go test with coverage"

go test $(go list ./... | grep -v integration | grep -v /vendor/ | grep -v /template/)  -race -coverprofile=coverage.txt -covermode=atomic