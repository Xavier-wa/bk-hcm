## ADDED Requirements

### Requirement: Subgraph checkpoint namespace uses strictly increasing turn counter
The system SHALL generate the subgraph checkpoint namespace as `<nodePrefix>_<turn>`, where `turn` is a monotonically increasing per-session, per-subgraph turn counter, instead of using the message count `len(parent.messages)`.

#### Scenario: Fresh entry into subgraph increments turn
- **WHEN** a subgraph node (`host_apply`) is entered for the first time in a session (no prior `subgraphInterrupt` for this node)
- **THEN** the system computes `turn = previous_turn + 1` from the persisted state, assigns child state `CfgKeyCheckpointNS = "host_apply_<turn>"`, and persists the new `turn` in the child state.

#### Scenario: Distinct turns produce distinct namespaces
- **WHEN** the host_apply subgraph is entered in two different turns of the same session
- **THEN** the two child states receive namespaces `host_apply_<N>` and `host_apply_<N+1>` with strictly increasing `N`, and the second entry does NOT resume the first entry's completed (`__end__`) checkpoint.

#### Scenario: Equal message count does not cause namespace collision
- **WHEN** two turns of host_apply entry happen to have the same parent message count
- **THEN** the namespaces still differ because they are derived from `turn`, not from `len(msgs)`, so no stale checkpoint is matched.

### Requirement: Same-turn re-entry reuses the original namespace
The system MUST keep the original namespace on in-turn re-entries (hitl resume within the same Run, or a resumed Run re-entering the subgraph node), so the `turn` counter is not incremented again.

#### Scenario: HITL interrupt resume reuses namespace
- **WHEN** a subgraph node raised a tool-call interrupt (`graph.Interrupt`) and the user resumes within the same session
- **THEN** `buildChildStateForAgentNode` applies `applyCheckpointResumeFields`, overriding `CfgKeyCheckpointNS` with the `subgraphInterrupt.childCheckpointNS` recorded at interrupt time, and the child resumes at the original `host_apply_<turn>` namespace without incrementing `turn`.

#### Scenario: Loop edge re-entry within one turn
- **WHEN** the graph re-enters the same subgraph node via a loop edge within a single turn
- **THEN** the namespace remains `host_apply_<turn>` (the interrupt-recorded namespace takes precedence) and no new `turn` is consumed.

### Requirement: Turn counter is persisted across runs
The `turn` counter MUST be persisted in the subgraph child state so that resumed runs do not reset it to zero and re-collide.

#### Scenario: Resumed run preserves turn
- **WHEN** a subgraph turn was persisted and the session is later resumed from a checkpoint
- **THEN** the next fresh entry reads the persisted `turn` (not zero) before incrementing, and namespaces remain strictly increasing across the resume boundary.

### Requirement: Multiple subgraphs use isolated namespace spaces
Each subgraph node (e.g. `host_apply`, `resource_query`) SHALL maintain an independent `turn` namespace space keyed by its `nodePrefix`.

#### Scenario: Different subgraphs do not share turn counters
- **WHEN** `host_apply` reaches turn 5 and `resource_query` is entered
- **THEN** `resource_query` starts its own counter (e.g. `resource_query_1`) and its namespace advancement does not affect `host_apply`'s counter.
