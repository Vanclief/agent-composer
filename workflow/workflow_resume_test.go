package workflow

import "testing"

func TestDownstreamOf(t *testing.T) {
	// a → b → d, a → c, e (independent)
	snapshot := &Snapshot{
		Nodes: map[string]NodeSnapshot{
			"a": {},
			"b": {InputBindings: map[string]Binding{
				"in": {Kind: BindingKindInstance, InstanceID: "a"},
			}},
			"c": {InputBindings: map[string]Binding{
				"in": {Kind: BindingKindInstance, InstanceID: "a"},
			}},
			"d": {InputBindings: map[string]Binding{
				"in": {Kind: BindingKindInstance, InstanceID: "b"},
			}},
			"e": {},
		},
	}

	downstream := DownstreamOf(snapshot, "b")
	for _, id := range []string{"b", "d"} {
		if !downstream[id] {
			t.Fatalf("%s should be downstream of b: %v", id, downstream)
		}
	}
	for _, id := range []string{"a", "c", "e"} {
		if downstream[id] {
			t.Fatalf("%s should not be downstream of b: %v", id, downstream)
		}
	}

	all := DownstreamOf(snapshot, "a")
	if len(all) != 4 || all["e"] {
		t.Fatalf("downstream of a should be a,b,c,d: %v", all)
	}
}
