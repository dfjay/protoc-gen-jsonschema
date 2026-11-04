module github.com/dfjay/protoc-gen-jsonschema/testdata

go 1.26.0

require (
	buf.build/go/protovalidate v1.4.0
	github.com/dfjay/protoc-gen-jsonschema v0.0.0
	github.com/dop251/goja v0.0.0-20261102140139-622d10077b6c
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	github.com/stretchr/testify v1.12.1
	google.golang.org/protobuf v1.36.12
	pgregory.net/rapid v1.3.0
	sigs.k8s.io/yaml v1.6.0
)

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.12-20260825204119-511051f7f437.1 // indirect
	cel.dev/cel-go v0.32.0 // indirect
	cel.dev/expr v0.25.3 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/dlclark/regexp2/v2 v2.11.0 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/exp v0.0.0-20260820142414-ca536658362e // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260819154853-08b0e4226688 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260819154853-08b0e4226688 // indirect
)

replace github.com/dfjay/protoc-gen-jsonschema => ../
