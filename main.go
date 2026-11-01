package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/modules"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

// version is set by -ldflags in release builds (see .goreleaser.yaml).
var version string

// getVersion returns the release version, or else the module version that
// `go install ...@vX.Y.Z` records in the binary, or "develop" for local builds.
func getVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return "develop"
}

const helpMessage = `Usage protoc-gen-jsonschema:
protoc --jsonschema_out=. (OPTIONS) *.proto

This plugin converts 'protocol buffer' formats to 'json-schema'. Support json-schema 'draft-04, draft-06, draft-07, draft-2019-09, draft-2020-12' versions.

Every schema needs an entrypoint message: the message the schema describes. Pass it with
--jsonschema_opt=entrypoint_message=<Message> or set it in the file with
option (dfjay.jsonschema.file) = {entrypoint_message: "<Message>"}.

EXAMPLE: Generate a schema for message Form in form.proto
protoc --jsonschema_out=. --jsonschema_opt=entrypoint_message=Form form.proto

EXAMPLE: Generate with yaml format
protoc --jsonschema_out=. --jsonschema_opt=output_file_suffix=.yaml *.proto

EXAMPLE: Shrink bytes for transfer over network
protoc --jsonschema_out=. --jsonschema_opt=pretty_json_output=false

EXAMPLE: I'd like to comply with the protobuf JSON mapping standard
protoc --jsonschema_out=. --jsonschema_opt=int64_as_string=true

EXAMPLE: Don't convert my proto field names to lowerCamelCase
protoc --jsonschema_out=. --jsonschema_opt=preserve_proto_field_names=true

EXAMPLE: I'm not satisfied with the plugin's options. I want to customize every fields
protoc -I path/to/protoc-gen-jsonschema/proto -I . --jsonschema_out=. your.proto

Please check https://github.com/dfjay/protoc-gen-jsonschema for more details.

FLAGS:
  --version  : print version
  --help     : print help
`

func main() {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "--version":
			fmt.Println(getVersion())
		case "--help":
			fmt.Print(helpMessage)
		}
		return
	}

	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "protoc-gen-jsonschema: %v\n", err)
		os.Exit(1)
	}
}

// run fails only when it cannot talk to protoc; other errors go into the
// response.
func run(in io.Reader, out io.Writer) error {
	input, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(input, req); err != nil {
		return fmt.Errorf("parse request (run this plugin through protoc): %w", err)
	}

	resp := modules.Generate(req, os.Stderr, os.Getenv("DEBUG") != "")
	output, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	_, err = out.Write(output)
	return err
}
