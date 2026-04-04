The DSL is getting strong on shape, wiring, composition, looping, and branching.

The DSL is already getting strong on shape and control flow:

- typed schemas
- reusable nodes
- explicit flow instances
- closed connector operations
- workflow composition
- loops
- conditionals

What it is still missing is the part that makes real workflows feel complete in production settings.

**Top Gap**

1. Failure semantics

- Real workflows need:
  - retry
  - timeout
  - on_error
  - fail_fast vs continue
- If the DSL cannot express failure behavior, production use will feel fragile.

**What I Would Prioritize**

If I had to choose the next things in order:

1. Failure policies

That would make the DSL feel more production-ready without making it bloated.

**What I Would Not Add**

- special nodes for voting, judging, reviewing, etc.
- custom connector mini-functions
- provider-specific behavior in the core DSL

The DSL should stay generic. The power should come from a small set of strong primitives, not a long list of named patterns.

So my view is: failure semantics are now the next major gap.
