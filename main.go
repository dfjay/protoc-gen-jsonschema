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

const helpMessage = `Usage: protoc --jsonschema_out=DIR [--jsonschema_opt=NAME=VALUE ...] FILE.proto

protoc-gen-jsonschema writes JSON Schemas of the ProtoJSON form of protobuf messages.
It supports JSON Schema draft-04, draft-06, draft-07, 2019-09 and 2020-12.

A schema is written for each entrypoint message. Name a top-level message of the file
by its simple name with --jsonschema_opt=entrypoint_message=<Message> or
option (dfjay.jsonschema.file) = {entrypoint_message: "<Message>"}, which writes
<file>.schema.json. Or mark any number of messages with
option (dfjay.jsonschema.message) = {entrypoint: true}, which writes
<file>.<Message>.schema.json for each.

EXAMPLES:
  # Schema of message Form in form.proto, written to form.schema.json (draft 2020-12)
  protoc -I . --jsonschema_out=. --jsonschema_opt=entrypoint_message=Form form.proto

  # YAML output
  protoc -I . --jsonschema_out=. --jsonschema_opt=entrypoint_message=Form \
    --jsonschema_opt=output_file_suffix=.schema.yaml form.proto

  # Another draft, compact JSON
  protoc -I . --jsonschema_out=. --jsonschema_opt=entrypoint_message=Form \
    --jsonschema_opt=draft=Draft07 --jsonschema_opt=pretty_json_output=false form.proto

  # Validate JSON or YAML that a ProtoJSON parser reads, such as config files
  protoc -I . --jsonschema_out=. --jsonschema_opt=entrypoint_message=Form \
    --jsonschema_opt=protojson_mode=Lenient form.proto

  # Custom annotations: import "dfjay/jsonschema/options.proto" in your file
  protoc -I . -I path/to/protoc-gen-jsonschema/proto --jsonschema_out=. form.proto

All options: https://github.com/dfjay/protoc-gen-jsonschema#plugin-options

FLAGS:
  --version  print version
  --help     print help
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
