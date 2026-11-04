#!/bin/bash

set -euxo pipefail

cd $(dirname $0)
proto_dirs=$(ls ../cases)

cd $(git rev-parse --show-toplevel)
go build -o ./testdata/hack/protoc-gen-jsonschema main.go

for proto_dir in $proto_dirs; do
  # Cases that expect the plugin to fail have no schema to write
  if [[ "$(yq eval '.expectFail' "./testdata/cases/$proto_dir/test.yaml")" == "true" ]]; then
    continue
  fi

  command="protoc \
  --plugin=protoc-gen-jsonschema=./testdata/hack/protoc-gen-jsonschema \
  --jsonschema_out=testdata/cases/$proto_dir "

  # Plugin options come from test.yaml, if there is one
  if [[ -f "./testdata/cases/$proto_dir/test.yaml" ]]; then
    # Read inputParameters from test.yaml
    parameters=$(yq eval '.inputParameters' ./testdata/cases/$proto_dir/test.yaml)

    # Skip when inputParameters is missing or empty
    if [[ "$parameters" != "null" && "$parameters" != "{}" ]]; then
        # Turn each key: value pair into --jsonschema_opt=<key>=<value>
        while IFS=":" read -r key value; do
            key=$(echo $key | tr -d ' "')
            value=$(echo $value | tr -d ' "')
            command="${command} --jsonschema_opt=${key}=${value}"
        done <<< "$parameters"
    fi
  fi

  command="$command \
  -I ./proto \
  -I testdata/cases/$proto_dir \
  -I testdata/include \
  testdata/cases/$proto_dir/*.proto
  "

  $command
done

rm ./testdata/hack/protoc-gen-jsonschema
