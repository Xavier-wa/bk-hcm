## MODIFIED Requirements

### Requirement: Graph entry runs intent recognition when intent is not host_apply

The agent graph SHALL use `scene_dispatch` as the entry point for a new run. `scene_dispatch` SHALL route to `intent_recognition` only when the session has no supported `session_tag` and no intent has been recognized this turn; it SHALL route directly to `llm` when the session has a supported `session_tag` (currently `host_apply`). `intent_recognition` itself SHALL NOT route to `llm` or `fallback` directly — it SHALL return to `scene_dispatch`.

#### Scenario: First message in an untagged session

- **WHEN** the user sends the first message and the session has no `session_tag`
- **THEN** the graph runs `scene_dispatch`, which routes to `intent_recognition`, which returns to `scene_dispatch` for the routing decision

#### Scenario: First message in a host_apply tagged session

- **WHEN** the user sends the first message and the session `session_tag` is `host_apply`
- **THEN** `scene_dispatch` routes directly to `llm` and `intent_recognition` is not invoked

### Requirement: host_apply routes to ReAct and skips intent recognition for the rest of the session

When `scene_dispatch` observes a supported intent for an untagged session (this-turn `StateKeyIntent=host_apply`), the system SHALL write back `session_tag=host_apply` to the session, persist `StateKeySessionTag=host_apply`, and route to the `llm` ReAct node. For all subsequent user messages in the same session after a `fallback` interrupt/resume, the system SHALL route directly to `llm` based on `StateKeySessionTag` without re-entering `intent_recognition`.

#### Scenario: Multi-turn host apply continues on llm

- **WHEN** the session `StateKeySessionTag` is `host_apply` and the user replies after the assistant asked a follow-up question
- **THEN** the graph resumes from `fallback` directly to `llm` and does not invoke `scene_dispatch` re-dispatch into `intent_recognition`

#### Scenario: scene_dispatch sets host_apply tag once and writes back

- **WHEN** intent recognition returns `host_apply` for a user message in an untagged session and returns to `scene_dispatch`
- **THEN** `scene_dispatch` writes `StateKeySessionTag=host_apply`, writes back `session_tag=host_apply` to the session, and routes to `llm`

### Requirement: Unsupported intents use fallback and re-run intent recognition on next turn

When `scene_dispatch` observes a this-turn intent other than `host_apply` (e.g. `chat`, `resource_query`, or unrecognized) for an untagged session, the system SHALL route to `fallback`, deliver the configured unsupported message, and on the next user message SHALL clear the this-turn `StateKeyIntent` and route back to `scene_dispatch` (which then re-enters `intent_recognition`). For a session whose `StateKeySessionTag` is supported (`host_apply`), `fallback` SHALL route directly to `llm` and SHALL NOT return to `scene_dispatch` or `intent_recognition`.

#### Scenario: Chat intent gets static reply then re-dispatches

- **WHEN** the this-turn intent is `chat` and the user sends another message after the fallback reply
- **THEN** resume clears `StateKeyIntent`, routes back to `scene_dispatch`, which routes to `intent_recognition` again before any further routing decision

#### Scenario: Resource query is not supported for ReAct

- **WHEN** the this-turn intent is `resource_query`
- **THEN** `scene_dispatch` routes to `fallback` with the resource-query unsupported message and does not enter `llm`

#### Scenario: Fallback keeps tagged session on llm

- **WHEN** the fallback node resumes with a new user message in a session whose `StateKeySessionTag` is `host_apply`
- **THEN** routing goes directly to `llm` and does not return to `scene_dispatch` or `intent_recognition`
