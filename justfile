bin := justfile_directory() / ".bin"

default:
    @just --list

# Everything CI runs
check: lint test

# gofmt, vet, golangci-lint
lint:
    test -z "$(gofmt -l .)" || { gofmt -l .; exit 1; }
    go vet ./...
    cd testdata && go vet ./...
    golangci-lint run
    cd testdata && golangci-lint run --config ../.golangci.yml

test:
    go test ./...
    cd testdata && go test ./...

# Regenerate pkg/proto/options.pb.go and options.md from the options proto
generate: _tools
    PATH="{{bin}}:$PATH" protoc -I proto \
        --go_out=. --go_opt=module=github.com/dfjay/protoc-gen-jsonschema \
        --doc_out=. --doc_opt=markdown,options.md \
        proto/dfjay/jsonschema/options.proto

# Regenerate the schemas in examples/ from examples/example.proto
examples: _plugin
    #!/usr/bin/env bash
    set -euo pipefail
    gen() {
        protoc --plugin=protoc-gen-jsonschema={{bin}}/protoc-gen-jsonschema \
            --jsonschema_out=examples "${@/#/--jsonschema_opt=}" \
            -I proto -I examples examples/example.proto
    }
    gen draft=Draft04 additional_properties=AlwaysTrue output_file_suffix=.json
    gen draft=Draft04 output_file_suffix=.schema.json
    gen draft=Draft04 output_file_suffix=.draft-04.json
    gen draft=Draft06 output_file_suffix=.draft-06.json
    gen draft=Draft07 output_file_suffix=.draft-07.json
    gen draft=Draft201909 output_file_suffix=.draft-2019-09.json
    gen draft=Draft202012 output_file_suffix=.draft-2020-12.json
    gen output_file_suffix=.yaml

# Accept changed expected schemas in testdata/cases: all, or the named case
golden case="":
    {{ if case == "" { "testdata/hack/generate_all_test_result.sh" } else { "testdata/hack/generate_test_result.sh " + case } }}

# Dump protoc's CodeGeneratorRequest for a testdata case into .bin/, for debugging
dump-request case="plugin-consist-output": _tools
    PATH="{{bin}}:$PATH" protoc --debug_out="{{bin}}/requests/{{case}}:." \
        -I proto -I testdata/cases/{{case}} testdata/cases/{{case}}/test.proto
    @echo {{bin}}/requests/{{case}}/code_generator_request.pb.bin

# Release dry run into dist/
snapshot:
    goreleaser release --snapshot --clean --skip=publish,sign

# protoc plugins at the versions go.mod pins, and protoc-gen-doc
_tools:
    go build -o {{bin}}/ google.golang.org/protobuf/cmd/protoc-gen-go github.com/lyft/protoc-gen-star/v2/protoc-gen-debug
    GOBIN={{bin}} go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@v1.5.1

_plugin:
    go build -o {{bin}}/protoc-gen-jsonschema .
