.PHONY: compile
compile:
	protoc pb/*.proto \
		--go_out=. \
		--go-grpc_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		--proto_path=.

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: build
build:
	go build -o bin/distsql .

.PHONY: test
test:
	go test ./...

.PHONY: cluster
cluster:
	docker compose up -d --build

.PHONY: generate
generate:
	go generate ./...