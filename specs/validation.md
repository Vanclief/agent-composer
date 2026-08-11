# Validation Rules

A valid workflow snapshot must satisfy all of the following.

## 5.1 Identity Rules

- `workflow_slug` must be present
- `workflow_version` must be present
- every `node_id` must be unique within the workflow snapshot
- port names must be unique within the scope of a single node snapshot or workflow boundary object

## 5.2 Wiring Rules

- every connection target must reference an existing node input port or workflow output port
- every input port may have at most one incoming connection
- workflow input ports may have zero or more outgoing connections
- workflow output ports may have at most one incoming connection
- a connection may only target an input endpoint
- a connection may only originate from an output endpoint

## 5.3 Port Readiness Rules

- a required input port must either have exactly one producer or have some future explicit default-binding mechanism if such a mechanism is introduced
- optional ports may be unbound
- workflow outputs marked required must have exactly one producer

## 5.4 Graph Rules

- the compiled graph must be acyclic
- every compiled edge must be derivable from the authoring model
- the compiled graph must contain no dangling endpoints

## 5.5 Execution Compatibility Rules

- a node snapshot must declare a valid kind
- the node snapshot kind must be compatible with its runtime configuration

Connector, inference, conditional, and loop-specific compatibility checks are intentionally deferred.
