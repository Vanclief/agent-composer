# DSL Examples

- [blueprint-plan-cycle.yaml](blueprint-plan-cycle.yaml): one blueprint-planning review cycle with revise, review, DSL validation, and triage
- [composition-article-summary-with-brief.yaml](composition-article-summary-with-brief.yaml): workflow composition plus one downstream inference step
- [composition-loop-iterative-code-repair.yaml](composition-loop-iterative-code-repair.yaml): initial code implementation followed by a composed `while` review and repair loop
- [conditional-boolean-routing-review.yaml](conditional-boolean-routing-review.yaml): conditional `if` node driven by a boolean output from an upstream inference node
- [connector-collect-binary-votes.yaml](connector-collect-binary-votes.yaml): connector `collect` example with raw vote collection
- [loop-and-connector-parallel-code-review.yaml](loop-and-connector-parallel-code-review.yaml): combined `foreach` loop and `concat` connector example
- [loop-foreach-section-summary.yaml](loop-foreach-section-summary.yaml): simple `foreach` loop with one iterated input and one pass-through input
- [loop-while-binary-consensus.yaml](loop-while-binary-consensus.yaml): `while` loop with one evolving state value and one boolean break output
- [pipeline-parallel-review-fix-cycle.yaml](pipeline-parallel-review-fix-cycle.yaml): one repair cycle with parallel reviewers, issue validation, and triage into next-step issues and pending questions
- [pipeline-summary-critique-revise.yaml](pipeline-summary-critique-revise.yaml): multi-step inference pipeline with summarize, critique, and revise
- [plan-new-blueprint.yaml](plan-new-blueprint.yaml): top-level blueprint planner that loops over review and DSL validation until ready or blocked
