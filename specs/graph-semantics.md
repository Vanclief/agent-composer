# Graph Semantics

## 3.1 Boundary Semantics

A workflow has explicit named input ports and explicit named output ports.

The workflow boundary participates in wiring as follows:

- workflow input ports are source endpoints and may feed node input ports
- workflow output ports are sink endpoints and must be fed by exactly one producer unless they are optional and intentionally unbound

This allows one workflow to be connected to another workflow at system level through input/output compatibility. Nested workflow execution is supported only through explicit composite nodes such as workflow composition, loops, and conditionals.

## 3.2 Authoring-Time Wiring

Connections are authored declaratively in the workflow blueprint.

That means:

- a flow instance input may declare the source it reads from
- a workflow output may declare the source it reads from
- the compiler resolves those declarations into normalized edges between explicit endpoints

This keeps the authoring format compact while preserving an explicit compiled graph model.

Simple nodes may omit explicit port definitions and rely on default `in` and `out` ports.

Named ports are required where multiple inbound or outbound paths must be distinguished.

## 3.3 Compiled Graph

Before execution, the workflow blueprint is normalized into a workflow snapshot.

Compilation must:

- resolve blueprint references
- resolve effective port shapes
- normalize all authored connections into explicit edges
- validate single-producer and acyclicity rules
- produce the workflow snapshot used by execution

The compiled graph derived from the workflow snapshot is the canonical execution graph.

Composite nodes such as loops and conditionals may own nested runtime targets, but they still appear as single nodes in the compiled parent graph.

## 3.4 Fan-Out and Fan-In

Fan-out is allowed.

One output endpoint may connect to multiple downstream input ports. This produces multiple edges that all reference the same output endpoint.

Fan-in is not implicit.

No input port may receive more than one incoming edge.

To combine multiple upstream values, the graph must introduce a connector node or future composite node with multiple distinct input ports. That connector then becomes the single producer for downstream consumers.

This means connectors do not bypass the one-producer rule. They obey it by declaring multiple separate input ports.

## 3.5 No Cycles, No Repeated Arrivals

The baseline graph is not reactive.

A value does not arrive on the same edge multiple times during one workflow execution.

A node does not re-fire in response to additional later arrivals during the same execution.

A node executes once when its required inputs are ready, and then it is done.

That is the core reason the authored graph remains a DAG rather than becoming a state machine.
