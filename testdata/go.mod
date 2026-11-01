module github.com/dfjay/protoc-gen-jsonschema/testdata

go 1.26.0

require (
	github.com/dfjay/protoc-gen-jsonschema v0.0.0
	github.com/stretchr/testify v1.12.1
	google.golang.org/protobuf v1.36.12
	sigs.k8s.io/yaml v1.6.0
)

require (
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
)

replace github.com/dfjay/protoc-gen-jsonschema => ../
