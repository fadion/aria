package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The exit status is a contract docs/compatibility.md makes: 0 ran, 1 did not,
// 2 is a usage mistake. It had nothing testing it and was wrong -- an unknown
// command printed the help and exited 0, because setting app.Action makes
// urfave/cli route unmatched arguments there instead of to CommandNotFound, so
// a mistyped subcommand in a script read as success.
//
// This builds the binary rather than calling into main(), because the thing
// under test is what the process returns to a shell.
func buildAria(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "aria")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the interpreter: %v\n%s", err, out)
	}
	return bin
}

func TestExitStatus(t *testing.T) {
	bin := buildAria(t)
	dir := t.TempDir()

	write := func(name, src string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	good := write("good.ari", "println(\"ok\")\n")
	runtimeBad := write("runtime.ari", "println(1 / 0)\n")
	resolveBad := write("resolve.ari", "let x = nope\n")
	parseBad := write("parse.ari", "let x = (\n")

	tests := []struct {
		name string
		args []string
		want int
	}{
		{"run a good file", []string{"run", good}, 0},
		{"check a good file", []string{"check", good}, 0},
		{"evaluate with -e", []string{"-e", "println(1)"}, 0},
		{"no arguments prints help", nil, 0},

		{"a runtime failure", []string{"run", runtimeBad}, 1},
		{"a name the resolver rejects", []string{"run", resolveBad}, 1},
		{"a parse failure", []string{"run", parseBad}, 1},
		{"check rejects a bad file", []string{"check", resolveBad}, 1},
		{"a file that is not there", []string{"run", filepath.Join(dir, "nope.ari")}, 1},
		{"a bad program via -e", []string{"-e", "let x = ("}, 1},

		{"an unknown command", []string{"bogus"}, 2},
		{"a mistyped command", []string{"rnu", good}, 2},
		{"an unknown flag", []string{"--nope"}, 2},
		{"run with no file", []string{"run"}, 2},
		{"check with no file", []string{"check"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(bin, tt.args...)
			out, err := cmd.CombinedOutput()
			got := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatalf("running %v: %v", tt.args, err)
				}
				got = exit.ExitCode()
			}
			if got != tt.want {
				t.Errorf("aria %s exited %d, want %d\n%s",
					strings.Join(tt.args, " "), got, tt.want, out)
			}
		})
	}
}

// A usage mistake has to say so. Exiting 2 silently would be as unhelpful as
// exiting 0, and the old behavior printed the whole help text, which buries the
// one line that matters.
func TestUsageMistakesSayWhat(t *testing.T) {
	bin := buildAria(t)

	for _, tt := range []struct{ args, want []string }{
		{[]string{"bogus"}, []string{"bogus", "doesn't exist"}},
		{[]string{"--nope"}, []string{"nope"}},
	} {
		out, _ := exec.Command(bin, tt.args...).CombinedOutput()
		for _, want := range tt.want {
			if !strings.Contains(string(out), want) {
				t.Errorf("aria %s: output does not mention %q:\n%s",
					strings.Join(tt.args, " "), want, out)
			}
		}
	}
}
