package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePkg создаёт временный пакет с файлом src.
func writePkg(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// readGen читает сгенерированный reset.gen.go.
func readGen(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "reset.gen.go"))
	if err != nil {
		t.Fatalf("reset.gen.go not generated: %v", err)
	}
	return string(data)
}

func TestRun_GeneratesForMarkedStruct(t *testing.T) {
	src := `package demo

// generate:reset
type A struct {
	I int
	S string
}

type B struct {
	X bool
}
`
	dir := writePkg(t, src)

	if err := run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}

	out := readGen(t, dir)

	if !strings.Contains(out, "func (rs *A) Reset()") {
		t.Errorf("expected Reset method for A, got:\n%s", out)
	}
	if strings.Contains(out, "func (rs *B) Reset()") {
		t.Errorf("B should not be generated (no marker), got:\n%s", out)
	}
	if !strings.Contains(out, "rs.I = 0") {
		t.Errorf("expected rs.I = 0, got:\n%s", out)
	}
	if !strings.Contains(out, `rs.S = ""`) {
		t.Errorf(`expected rs.S = "", got:\n%s`, out)
	}
}

func TestRun_IgnoresUnmarkedPackages(t *testing.T) {
	dir := writePkg(t, `package demo

type Only struct{ X int }
`)
	if err := run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "reset.gen.go")); !os.IsNotExist(err) {
		t.Errorf("reset.gen.go should not be created for unmarked package")
	}
}

func TestResetRules_PrimitivesPointersSlicesMaps(t *testing.T) {
	src := `package demo

// generate:reset
type Sample struct {
	I    int
	Str  string
	B    bool
	F    float64
	StrP *string
	S    []int
	M    map[string]string
	Arr  [2]int
}
`
	dir := writePkg(t, src)
	if err := run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := readGen(t, dir)

	for _, want := range []string{
		"rs.I = 0",
		`rs.Str = ""`,
		"rs.B = false",
		"rs.F = 0",
		"if rs.StrP != nil {",
		`(*rs.StrP) = ""`,
		"rs.S = rs.S[:0]",
		"clear(rs.M)",
		"for i := range rs.Arr {",
		"rs.Arr[i] = 0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in generated code, got:\n%s", want, out)
		}
	}
}

func TestResetRules_NestedStructWithReset(t *testing.T) {
	src := `package demo

// generate:reset
type Child struct {
	X int
}

// generate:reset
type Parent struct {
	C  Child
	CP *Child
}
`
	dir := writePkg(t, src)
	if err := run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := readGen(t, dir)

	if !strings.Contains(out, "rs.C.Reset()") {
		t.Errorf("expected nested Child to call Reset(), got:\n%s", out)
	}
	if !strings.Contains(out, "if rs.CP != nil {") {
		t.Errorf("expected nil-check on *Child, got:\n%s", out)
	}
	if !strings.Contains(out, "(*rs.CP).Reset()") {
		t.Errorf("expected (*rs.CP).Reset(), got:\n%s", out)
	}
}

func TestResetRules_NestedStructWithoutReset(t *testing.T) {
	src := `package demo

type Plain struct {
	A string
}

// generate:reset
type Holder struct {
	P Plain
}
`
	dir := writePkg(t, src)
	if err := run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := readGen(t, dir)

	if !strings.Contains(out, "rs.P.A = \"\"") {
		t.Errorf("expected inlined field reset for Plain, got:\n%s", out)
	}
}

func TestResetRules_DurationReset(t *testing.T) {
	src := `package demo

import "time"

// generate:reset
type Cfg struct {
	T time.Duration
}
`
	dir := writePkg(t, src)
	if err := run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := readGen(t, dir)

	if !strings.Contains(out, "rs.T = 0") {
		t.Errorf("expected Duration reset to 0, got:\n%s", out)
	}
}

func TestResetRules_NilReceiverGuard(t *testing.T) {
	src := `package demo

// generate:reset
type X struct{ A int }
`
	dir := writePkg(t, src)
	if err := run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := readGen(t, dir)

	if !strings.Contains(out, "if rs == nil {") {
		t.Errorf("expected nil receiver guard, got:\n%s", out)
	}
}

func TestRun_SkipsTestAndGeneratedFiles(t *testing.T) {
	dir := t.TempDir()

	src := `package demo

// generate:reset
type Real struct{ A int }
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	// _test.go файл с маркером не должен учитываться.
	if err := os.WriteFile(filepath.Join(dir, "extra_test.go"), []byte(`package demo

// generate:reset
type TestOnly struct{ B int }
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := readGen(t, dir)

	if strings.Contains(out, "func (rs *TestOnly) Reset()") {
		t.Errorf("test-only struct should not be generated, got:\n%s", out)
	}
	if !strings.Contains(out, "func (rs *Real) Reset()") {
		t.Errorf("expected Real to be generated, got:\n%s", out)
	}
}

func TestGenFile_Header(t *testing.T) {
	src := `package demo

// generate:reset
type A struct{ I int }
`
	dir := writePkg(t, src)
	if err := run(dir); err != nil {
		t.Fatalf("run: %v", err)
	}
	out := readGen(t, dir)

	for _, want := range []string{
		"// Code generated by go generate; DO NOT EDIT.",
		"package demo",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected header %q, got:\n%s", want, out)
		}
	}
}