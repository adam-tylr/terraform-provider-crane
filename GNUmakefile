default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc: test-registry-up
	TF_ACC=1 SOURCE_REGISTRY=localhost:5001 go test -v -cover -timeout 120m ./...; \
	status=$$?; \
	$(MAKE) test-registry-down; \
	exit $$status

test-registry-up:
	@echo "Starting local container registry..."
	@docker run -d -p 5001:5000 -e REGISTRY_STORAGE_DELETE_ENABLED=true --name crane-test-registry registry:2 || true

test-registry-down:
	@echo "Stopping local container registry..."
	@docker rm -f crane-test-registry || true

.PHONY: fmt lint test testacc build install generate test-registry-up test-registry-down

