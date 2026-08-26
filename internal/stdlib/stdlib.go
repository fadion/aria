// Package stdlib holds Aria's standard library, written in Aria.
//
// The sources are embedded with go:embed. The original kept them as Go string
// literals with a comment saying alternatives existed but would need a
// dependency — go:embed has been in the standard library since Go 1.16, and it
// means the modules are ordinary .ari files that an editor can highlight and the
// test suite can run through the real parser.
//
// Four things differ from the version under library/, all of them consequences
// of rulings in docs/architecture.md that the resolver surfaced:
//
//	Enum.insert   built a new array instead of mutating its parameter
//	Dict.insert   the same
//	Dict.update   the same
//	Enum.random   called `rand`, which does not exist; it is runtime_rand
package stdlib

import (
	"embed"
	"path"
	"sort"
	"strings"
)

//go:embed *.ari
var files embed.FS

// A Module is one standard library unit.
type Module struct {
	// Name is the module name, which is also its file's base name.
	Name string
	// Path is the embedded file name, used as the source file name so a
	// diagnostic inside the library points at a real place.
	Path string
	Src  string
}

// Modules returns the standard library in a stable order.
//
// Order matters: a module that names another must be evaluated after it, and a
// fixed order makes any failure reproducible.
func Modules() []Module {
	entries, err := files.ReadDir(".")
	if err != nil {
		panic("stdlib: " + err.Error())
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && path.Ext(e.Name()) == ".ari" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	out := make([]Module, 0, len(names))
	for _, name := range names {
		src, err := files.ReadFile(name)
		if err != nil {
			panic("stdlib: " + err.Error())
		}
		out = append(out, Module{
			Name: moduleName(string(src)),
			Path: "<stdlib>/" + name,
			Src:  string(src),
		})
	}
	return out
}

// Names lists the module names the standard library declares, for predeclaring
// them before a program that uses them is resolved.
func Names() []string {
	mods := Modules()
	out := make([]string, 0, len(mods))
	for _, m := range mods {
		if m.Name != "" {
			out = append(out, m.Name)
		}
	}
	return out
}

func moduleName(src string) string {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if after, found := strings.CutPrefix(line, "module "); found {
			return strings.TrimSpace(after)
		}
	}
	return ""
}
