package workflow

// DownstreamOf returns the target node plus every node that
// transitively consumes its outputs — the set that must re-run when
// resuming a workflow from that node.
func DownstreamOf(snapshot *Snapshot, target string) map[string]bool {
	dependents := map[string][]string{}
	for instanceID, node := range snapshot.Nodes {
		for _, binding := range node.InputBindings {
			if binding.Kind == BindingKindInstance && binding.InstanceID != "" {
				dependents[binding.InstanceID] = append(
					dependents[binding.InstanceID],
					instanceID,
				)
			}
		}
	}

	result := map[string]bool{target: true}
	queue := []string{target}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, dependent := range dependents[current] {
			if !result[dependent] {
				result[dependent] = true
				queue = append(queue, dependent)
			}
		}
	}

	return result
}
