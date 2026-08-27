package interp

import (
	"sort"
	"testing"

	"github.com/fadion/aria/internal/resolver"
	"github.com/fadion/aria/internal/source"
)

// The resolver keeps its own list of builtin names so it can reject an
// undefined one before the program runs, and the evaluator installs the real
// functions. Nothing tied the two together, so a builtin added to one and not
// the other compiled fine and failed at the worst moment: adding runtime_code
// and runtime_from_code to the evaluator alone made the standard library itself
// fail to resolve, with "internal error: the standard library failed to
// compile" and no hint that a list elsewhere was short.
func TestBuiltinListsAgree(t *testing.T) {
	i := New(source.NewFile("test.ari", nil), nil)

	installed := []string{}
	for name, v := range i.globals.vars {
		if _, ok := v.(*Builtin); ok {
			installed = append(installed, name)
		}
	}

	declared := map[string]bool{}
	for _, name := range resolver.Builtins {
		declared[name] = true
	}
	have := map[string]bool{}
	for _, name := range installed {
		have[name] = true
	}

	var missing, extra []string
	for _, name := range installed {
		if !declared[name] {
			missing = append(missing, name)
		}
	}
	for _, name := range resolver.Builtins {
		if !have[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)

	if len(missing) > 0 {
		t.Errorf("installed by the evaluator but not in resolver.Builtins, so the "+
			"resolver will reject any call to them: %v", missing)
	}
	if len(extra) > 0 {
		t.Errorf("in resolver.Builtins but never installed, so a call resolves and "+
			"then fails at runtime: %v", extra)
	}
}
