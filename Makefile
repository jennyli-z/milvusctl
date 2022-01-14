# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

fmt: ## Run go fmt against code
	go fmt ./...

vet: ## Run go vet against code
	go vet ./...