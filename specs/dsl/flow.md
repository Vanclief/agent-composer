# Flow DSL

Flow instances are authored under `flow.instances`.

Shape:

```yaml
flow:
  instances:
    <instance_id>:
      node: string
      inputs: object?
```

Where:

- `<instance_id>` is the public authored instance id
- `node` selects which local node definition the instance uses
- `inputs` declares the sources bound to the instance input ports

For ordinary nodes, `<instance_id>` becomes the compiled `node_id`.

For workflow nodes, `<instance_id>` becomes the namespace prefix for the compiled child graph and the authored binding prefix for child workflow outputs.

## Authored Bindings

Connections are declared by binding each input to a source.

Examples:

```yaml
workflow_input.article_text
instance.summarize_article.out
instance.summarize.final_summary
instance.aggregate_validated_issues.out
```

Rules:

- each authored binding must resolve to a real producer
- each input port may have at most one producer
- fan-out is allowed
- fan-in must be made explicit through a connector or other composite node
- if an instance uses a workflow node, its valid input names come from the referenced child workflow inputs
- if an instance uses a workflow node, its valid output names come from the referenced child workflow outputs

The compiler resolves these authored bindings into explicit `Edge` records in the internal model. Workflow instances are expanded into namespaced child nodes and edges before the final flat graph is emitted.

## Example

```yaml
flow:
  instances:
    reviewer_a:
      node: code_reviewer
      inputs:
        branch: workflow_input.compare_branch

    validate_reviewer_a_issues:
      node: issue_validation_worker
      inputs:
        items: instance.reviewer_a.out
```
