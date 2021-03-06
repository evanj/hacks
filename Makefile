# Generates protocol buffer messages and downloading required tools
BUILD_DIR:=build
PROTOC:=$(BUILD_DIR)/bin/protoc
PROTOC_GEN_GO:=$(BUILD_DIR)/protoc-gen-go

all: protodecode/protodemo/demo.pb.go
	goimports -l -w .
	go test -race ./...
	go vet ./...
	golint ./...
	# ignore protocol buffers for staticcheck
	go list ./... | grep -v '/protodemo' | xargs staticcheck
	go mod tidy
	find . -type f | grep '\.proto$$' | xargs clang-format -Werror -i '-style={ColumnLimit: 100}'

protodecode/protodemo/demo.pb.go: protodecode/protodemo/demo.proto $(PROTOC) $(PROTOC_GEN_GO)
	$(PROTOC) --plugin=$(PROTOC_GEN_GO) --go_out=plugins=grpc,paths=source_relative:. $<

# download protoc to a temporary tools directory
$(PROTOC): buildtools/getprotoc.go | $(BUILD_DIR)
	go run $< --outputDir=$(BUILD_DIR)

$(PROTOC_GEN_GO): | $(BUILD_DIR)
	go build --mod=readonly -o $@ github.com/golang/protobuf/protoc-gen-go

$(BUILD_DIR):
	mkdir -p $@

clean:
	$(RM) -r build
