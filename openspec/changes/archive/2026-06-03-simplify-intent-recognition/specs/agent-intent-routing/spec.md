## ADDED Requirements

### Requirement: Graph entry runs intent recognition when intent is not host_apply

The agent graph SHALL use `intent_recognition` as the entry point for a new run when `StateKeyIntent` is not `host_apply`.

#### Scenario: First message in a new session

- **WHEN** the user sends the first message and graph state has no `host_apply` intent
- **THEN** the graph runs the `intent_recognition` node before routing to `llm` or `fallback`

### Requirement: host_apply routes to ReAct and skips intent recognition for the rest of the session

When intent recognition classifies the user intent as `host_apply`, the system SHALL persist `StateKeyIntent` as `host_apply` and route to the `llm` ReAct node. For all subsequent user messages in the same session after a `fallback` interrupt/resume, the system SHALL route directly to `llm` without running `intent_recognition`.

#### Scenario: Multi-turn host apply continues on llm

- **WHEN** the session intent is `host_apply` and the user replies after the assistant asked a follow-up question
- **THEN** the graph resumes from `fallback` to `llm` and does not invoke `intent_recognition`

#### Scenario: Intent recognition sets host_apply once

- **WHEN** intent recognition returns `host_apply` for a user message
- **THEN** the graph writes `StateKeyIntent=host_apply` and routes to `llm`

### Requirement: Unsupported intents use fallback and re-run intent recognition on next turn

When intent recognition classifies an intent other than `host_apply` (e.g. `chat`, `resource_query`), the system SHALL route to `fallback`, deliver the configured unsupported message, and on the next user message SHALL route to `intent_recognition` again.

#### Scenario: Chat intent gets static reply then re-classifies

- **WHEN** intent recognition returns `chat` and the user sends another message after the fallback reply
- **THEN** the graph runs `intent_recognition` again before any routing decision

#### Scenario: Resource query is not supported for ReAct

- **WHEN** intent recognition returns `resource_query`
- **THEN** the graph routes to `fallback` with the resource-query unsupported message and does not enter `llm`

### Requirement: finish_intent_task is not used

The system SHALL NOT expose or register a `finish_intent_task` tool, and the LLM system prompt SHALL NOT instruct the model to call it.

#### Scenario: Tool list for graph agent

- **WHEN** the graph agent is built with skill and HITL tools
- **THEN** `finish_intent_task` is absent from the tool set and tool node callbacks

### Requirement: Intent task status enum is not used for routing

The system SHALL NOT use `StateKeyIntentTaskStatus` or `IntentTaskStatus` values (`idle`, `in_progress`, `completed`) for conditional edges or intent node output.

#### Scenario: Fallback routing uses intent only

- **WHEN** the fallback node resumes with a new user message
- **THEN** routing checks only `StateKeyIntent` (and not `intent_task_status`) to choose between `llm` and `intent_recognition`
