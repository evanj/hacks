# Generates protocol buffer messages and downloading required tools
BUILD_DIR:=build
PROTOC:=$(BUILD_DIR)/bin/protoc
PROTOC_GEN_GO:=$(BUILD_DIR)/protoc-gen-go
NODE_DIR:=$(BUILD_DIR)/node

# always execute all/clean when asked
.PHONY: all clean

all: protodecode/protodemo/demo.pb.go $(NODE_DIR)
	# archlevel has architecture-specific code in it: make sure it compiles
	GOOS=linux go build -o /dev/null ./archlevel
	GOOS=darwin go build -o /dev/null ./archlevel

	goimports -l -w .
	CGO_ENABLED=1 go test -race -shuffle=on -count=2 ./...
	go vet ./...
	# ignore protocol buffers for staticcheck
	go list ./... | grep -v '/protodemo$$' | xargs staticcheck --checks=all
	go mod tidy
	find . -type f | grep '\.proto$$' | xargs clang-format -Werror -i '-style={ColumnLimit: 100}'

	GOPATH=$(shell go env GOPATH) govulncheck ./...

	echo "typescript version:"
	go run ./runtypescript --nodeDir=$(NODE_DIR) -- --version

$(NODE_DIR): runtypescript/runtypescript.go | $(BUILD_DIR)
	$(RM) -r $@
	go run $< --verbose --nodeDir=$@ -- --version

protodecode/protodemo/demo.pb.go: protodecode/protodemo/demo.proto $(PROTOC) $(PROTOC_GEN_GO)
	$(PROTOC) --plugin=$(PROTOC_GEN_GO) --go_out=paths=source_relative:. $<

# download protoc to a temporary tools directory
$(PROTOC): getprotoc/getprotoc.go | $(BUILD_DIR)
	go run $< --outputDir=$(BUILD_DIR)

# go install uses the version of protoc-gen-go specified by go.mod ... I think
$(PROTOC_GEN_GO): go.mod | $(BUILD_DIR)
	GOBIN=$(realpath $(BUILD_DIR)) go install google.golang.org/protobuf/cmd/protoc-gen-go

$(BUILD_DIR):
	mkdir -p $@

clean:
	$(RM) -r build
