package testdata

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dfjay/protoc-gen-jsonschema/pkg/modules"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
	"sigs.k8s.io/yaml"
)

type Testcase struct {
	// Testcase Identifier
	Name string `json:"name"`

	// Testcase Description
	Description                 string `json:"description"`
	ExpectedBehaviorDescription string `json:"expectedBehaviorDescription"`

	// Testcase Input
	InputFiles      []string          `json:"inputFiles"`
	InputParameters map[string]string `json:"inputParameters"`

	// Expected result
	ExpectResultFiles  []string `json:"expectResultFiles"`
	ExpectResultIsNull bool     `json:"expectResultIsNull"`
	ExpectFail         bool     `json:"expectFail"`

	// Why TestRoundTrip skips the case.
	SkipRoundTrip string `json:"skipRoundTrip"`

	// Check the schema against protovalidate in TestProtovalidate.
	CheckProtovalidate bool `json:"checkProtovalidate"`
}

const testdataDir = "./cases"
const optionsDir = "../proto"

// includeDir holds third-party protos the cases import, such as protovalidate.
const includeDir = "./include"

// TestPlugin compares the output of each case byte for byte with the files
// `just golden` writes through the real protoc.
func TestPlugin(t *testing.T) {
	optionsImportPath, err := filepath.Abs(optionsDir)
	if err != nil {
		t.Fatal(err)
	}

	forEachCase(t, func(t *testing.T, tc *Testcase, dirPath string) {
		req := buildRequest(t, tc, dirPath, optionsImportPath)

		resp := modules.Generate(req, nil, false)

		if tc.ExpectFail {
			if resp.Error == nil {
				t.Errorf("Expect failed, but got success")
			}
			return
		}
		if resp.Error != nil {
			t.Fatalf("Expect success, but the plugin failed: %s", resp.GetError())
		}

		if tc.ExpectResultIsNull {
			if len(resp.File) != 0 {
				t.Errorf("Expect result is null, but got %d of results", len(resp.File))
			}
			return
		}

		if len(resp.File) != len(tc.ExpectResultFiles) {
			t.Fatalf("Expect %d of results, but got %d of results", len(tc.ExpectResultFiles), len(resp.File))
		}
		for _, file := range resp.File {
			if !slices.Contains(tc.ExpectResultFiles, file.GetName()) {
				t.Fatalf("unexpected output file %s, want one of %v", file.GetName(), tc.ExpectResultFiles)
			}
			expected, err := os.ReadFile(filepath.Join(dirPath, file.GetName()))
			if err != nil {
				t.Fatalf("unexpected output file %s: %v", file.GetName(), err)
			}
			require.Equal(t, string(expected), file.GetContent(), "%s: %s differs from the expected file", tc.Name, file.GetName())
			if _, err := compileSchema(file.GetName(), file.GetContent()); err != nil {
				t.Errorf("%s is not a valid schema: %v", file.GetName(), err)
			}
		}
	})
}

func forEachCase(t *testing.T, f func(t *testing.T, tc *Testcase, dirPath string)) {
	dirs, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range dirs {
		tc, err := loadTestcase(testdataDir, dir)
		if err != nil {
			t.Fatal(err)
		}
		dirPath := filepath.Join(testdataDir, dir.Name())
		t.Run(dir.Name(), func(t *testing.T) { f(t, tc, dirPath) })
	}
}

// buildRequest builds the request protoc would send, from a descriptor set
// with imports and comments.
func buildRequest(t *testing.T, tc *Testcase, dirPath, optionsImportPath string) *pluginpb.CodeGeneratorRequest {
	t.Helper()
	descriptorSet := filepath.Join(t.TempDir(), "descriptor_set.pb")
	args := []string{
		"-I", dirPath,
		"-I", optionsImportPath,
		"-I", includeDir,
		"--include_imports",
		"--include_source_info",
		"--descriptor_set_out=" + descriptorSet,
	}
	for _, f := range tc.InputFiles {
		args = append(args, filepath.Join(dirPath, f))
	}
	var stderr bytes.Buffer
	cmd := exec.Command("protoc", args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("protoc %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}

	buf, err := os.ReadFile(descriptorSet)
	if err != nil {
		t.Fatal(err)
	}
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(buf, set); err != nil {
		t.Fatal(err)
	}

	var params []string
	for _, name := range slices.Sorted(maps.Keys(tc.InputParameters)) {
		params = append(params, name+"="+tc.InputParameters[name])
	}
	return &pluginpb.CodeGeneratorRequest{
		FileToGenerate: tc.InputFiles,
		Parameter:      new(strings.Join(params, ",")),
		ProtoFile:      set.GetFile(),
	}
}

func loadTestcase(parentDir string, dir os.DirEntry) (*Testcase, error) {
	if !dir.IsDir() {
		return nil, fmt.Errorf("dir %s is not directory", dir.Name())
	}

	dirPath := filepath.Join(parentDir, dir.Name())

	buf, err := os.ReadFile(filepath.Join(dirPath, "test.yaml"))
	if err != nil {
		return nil, err
	}
	testcase := &Testcase{}
	if err := yaml.Unmarshal(buf, testcase); err != nil {
		return nil, err
	}
	return testcase, nil
}

// TestSupportedEditions checks what the plugin tells protoc it supports:
// protoc refuses editions files otherwise.
func TestSupportedEditions(t *testing.T) {
	resp := modules.Generate(&pluginpb.CodeGeneratorRequest{}, nil, false)
	require.NotZero(t, resp.GetSupportedFeatures()&uint64(pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS))
	require.Equal(t, int32(descriptorpb.Edition_EDITION_PROTO2), resp.GetMinimumEdition())
	require.Equal(t, int32(descriptorpb.Edition_EDITION_2024), resp.GetMaximumEdition())
}
