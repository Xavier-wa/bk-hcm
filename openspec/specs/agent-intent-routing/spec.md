## Requirements

### Requirement: Graph entry runs intent recognition when intent is not host_apply

The agent graph SHALL use `scene_dispatch` as the entry point for a new run. `scene_dispatch` SHALL classify the user's intent **in-process** on every execution by calling `intent.Classify`, regardless of whether the session already carries a supported `session_tag`. The graph SHALL NOT contain a separate `intent_recognition` node, and therefore SHALL NOT contain a `scene_dispatch → intent_recognition → scene_dispatch` round trip.

Because `scene_dispatch` only executes at a turn boundary (a run resuming from an in-subgraph HITL interrupt re-enters that subgraph node directly), classification SHALL happen exactly once per run.

#### Scenario: First message in an untagged session

- **WHEN** the user sends the first message and the session has no `session_tag`
- **THEN** `scene_dispatch` classifies the intent in-process and routes to the matching scene subgraph, or to `fallback` when the intent is not a supported scene

#### Scenario: First message in a host_apply tagged session

- **WHEN** the user sends the first message and the session `session_tag` is `host_apply`
- **THEN** `scene_dispatch` still classifies the intent in-process, then routes to `host_apply` unless the classification names a different supported scene

#### Scenario: No intent_recognition node exists in the graph

- **WHEN** `BuildGraph` finishes compiling the main graph
- **THEN** no node named `intent_recognition` is registered, and `MainGraphAgentNode` no longer enumerates it

### Requirement: host_apply routes to ReAct and skips intent recognition for the rest of the session

When `scene_dispatch` classifies a supported scene for an untagged session, the system SHALL commit `StateKeySessionTag` to that scene and route to the corresponding scene subgraph, and SHALL write the tag back to the session after the run completes.

For subsequent user messages in the same session, the system SHALL re-classify at each turn boundary rather than routing on `StateKeySessionTag` alone: a supported classification that differs from the current tag SHALL switch the session to the new scene (see `agent-scene-switch`); any other classification SHALL keep the session on its current tag.

#### Scenario: Multi-turn host apply continues on host_apply subgraph

- **WHEN** the session `StateKeySessionTag` is `host_apply` and the user replies to a follow-up question with an in-scene message
- **THEN** classification returns `host_apply` (or a non-scene intent) and the graph routes back into the `host_apply` subgraph

#### Scenario: scene_dispatch sets host_apply tag once and writes back

- **WHEN** classification returns `host_apply` for a user message in an untagged session
- **THEN** `scene_dispatch` writes `StateKeySessionTag=host_apply` and routes to the `host_apply` subgraph; the tag is persisted to the session after the run completes

#### Scenario: Supported classification differing from the tag switches scene

- **WHEN** the session `StateKeySessionTag` is `host_apply` and classification returns `resource_query` at a turn boundary
- **THEN** `scene_dispatch` commits `StateKeySessionTag=resource_query` and routes to the `resource_query` subgraph within the same run

### Requirement: Unsupported intents use fallback and re-run intent recognition on next turn

When `scene_dispatch` classifies a non-scene intent (e.g. `chat` or unrecognized) for an **untagged** session, the system SHALL route to `fallback` and deliver the configured unsupported message. For a session whose `StateKeySessionTag` is a supported scene, a non-scene classification SHALL keep the session on that scene and route into its subgraph rather than to `fallback`.

`fallback` SHALL always return to `scene_dispatch` on resume so that the next turn is re-classified. The decision of whether `fallback` rebuilds the message history on resume SHALL be driven by `StateKeySessionTag` (unsupported/empty tag ⇒ rebuild), not by any per-turn intent state key.

#### Scenario: Chat intent in an untagged session gets static reply then re-dispatches

- **WHEN** the classification is `chat` for an untagged session and the user sends another message after the fallback reply
- **THEN** resume routes back to `scene_dispatch`, which classifies again before any further routing decision

#### Scenario: Chat intent in a tagged session stays in scene

- **WHEN** the classification is `chat` for a session whose `StateKeySessionTag` is `host_apply`
- **THEN** the graph routes into the `host_apply` subgraph and the unsupported message is not delivered

#### Scenario: Fallback history rebuild keyed on session_tag

- **WHEN** the fallback node resumes with a new user message
- **THEN** it rebuilds the assistant/user message tail only when `StateKeySessionTag` is empty or not a supported scene

### Requirement: finish_intent_task is not used

The system SHALL NOT expose or register a `finish_intent_task` tool, and the LLM system prompt SHALL NOT instruct the model to call it.

#### Scenario: Tool list for graph agent

- **WHEN** the graph agent is built with skill and HITL tools
- **THEN** `finish_intent_task` is absent from the tool set and tool node callbacks

### Requirement: Intent task status enum is not used for routing

The system SHALL NOT use `StateKeyIntentTaskStatus` or `IntentTaskStatus` values (`idle`, `in_progress`, `completed`) for conditional edges or routing decisions.

#### Scenario: Fallback routing uses session_tag only

- **WHEN** the fallback node resumes with a new user message
- **THEN** routing checks only `StateKeySessionTag` (and neither `intent_task_status` nor a per-turn intent key)
