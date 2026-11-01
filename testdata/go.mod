module github.com/dfjay/protoc-gen-jsonschema/testdata

go 1.26.0

require (
	github.com/dfjay/protoc-gen-jsonschema v0.0.0
	github.com/stretchr/testify v1.12.1
	google.golang.org/protobuf v1.36.12
	k8s.io/apimachinery v0.37.2
)

require (
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	sigs.k8s.io/json v0.0.0-20260909141634-11ed52e25bc5 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace github.com/dfjay/protoc-gen-jsonschema => ../
