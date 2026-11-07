# flowise

Flowise is a Node.js visual agent and chatflow builder (`npx flowise start`, Docker); its
Agentflow v2 is the second-generation agent canvas. It came into the catalog from the agent
framework sweep for [issue #64](https://github.com/seanthegeek/doubleagent/issues/64); see
[`research/frameworks.md`](frameworks.md) for the frameworks that got no catalog line. Its
parser is in the backlog ([issue 68](https://github.com/seanthegeek/doubleagent/issues/68)).

## Source and evidence

[FlowiseAI/Flowise](https://github.com/FlowiseAI/Flowise), commit
[`9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb`](https://github.com/FlowiseAI/Flowise/commit/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb).
Licence: Apache 2.0, except `packages/server/src/enterprise`, which is
commercial
([LICENSE.md:1-7](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/LICENSE.md#L1-L7)).
All claims are from source.

Flowise executes tools on the host. Custom JavaScript functions and tools
run in-process in a vm2 `NodeVM` unless `E2B_APIKEY` is set
([components utils.ts:1594-1606](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/src/utils.ts#L1594-L1606),
[1791](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/src/utils.ts#L1791)),
and MCP stdio servers are spawned on the host, with an allow-list for
custom scripts
([MCP/core.ts:117](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/nodes/tools/MCP/core.ts#L117),
[248-258](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/nodes/tools/MCP/core.ts#L248-L258)).

### Observed

- No real install: nothing was installed or run.

### Source reviews

- 2026-10-19: HEAD is still
  [`9291856`](https://github.com/FlowiseAI/Flowise/commit/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb),
  re-read from source for permission, model, past-use, sync and deletion records. It added
  [Database detail](#database-detail) and [Scratch directories](#scratch-directories).

## Collection

### Per-user storage

The home is `HOME`, or `USERPROFILE` on Windows
([server utils/index.ts:114-125](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/index.ts#L114-L125)),
so `~/.flowise` is used on every OS.

| Path | Holds | Default or opt-in |
| --- | --- | --- |
| `.flowise/database.sqlite` | all app data: tables `chat_message` (`content`, `usedTools`, `agentReasoning`), `execution` (`executionData`), `credential` (`encryptedData`), `chat_flow`, `tool` (custom tool code), `custom_mcp_server`, and `login_sessions` ([DataSource.ts:16-30](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/DataSource.ts#L16-L30), [90-99](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/DataSource.ts#L90-L99), [Init.ts:6-16](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/database/migrations/sqlite/1693835579790-Init.ts#L6-L16), [AddExecutionEntity.ts:6](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/database/migrations/sqlite/1738090872625-AddExecutionEntity.ts#L6), [ChatMessage.ts:25-38](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/database/entities/ChatMessage.ts#L25-L38), [SessionPersistance.ts:88-97](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/enterprise/middleware/passport/SessionPersistance.ts#L88-L97)) | default (`DATABASE_TYPE` unset or `sqlite`) |
| `.flowise/encryption.key` | AES key for `credential.encryptedData` ([utils/index.ts:1580-1588](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/index.ts#L1580-L1588)) | default |
| `.flowise/token_hash_secret.key`, `express_session_secret.key`, `jwt_auth_token_secret.key`, `jwt_refresh_token_secret.key` | auth signing secrets ([authSecrets.ts:29-56](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/enterprise/utils/authSecrets.ts#L29-L56), [utils/index.ts:1669-1671](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/index.ts#L1669-L1671), [1726-1735](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/index.ts#L1726-L1735)) | default |
| `.flowise/storage`, `.flowise/uploads` | chat uploads and generated files ([storageUtils.ts:50-53](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/src/storageUtils.ts#L50-L53), [utils/index.ts:2018-2022](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/index.ts#L2018-L2022)) | default |
| `.flowise/vectorstore` | Faiss and SimpleStore indexes ([validator.ts:213-224](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/src/validator.ts#L213-L224)) | default when the node is used |

Logs are not under the home. They go to `<install>/packages/server/logs`
(or `LOG_PATH`), which for `npx` is inside the npx cache
([utils/config.ts:10](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/config.ts#L10),
[services/log/index.ts:73-75](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/services/log/index.ts#L73-L75)).

#### Scratch directories

Flowise has no agent working directory. Chat
uploads and generated files go to per-chatflow, per-chat directories
under `.flowise/storage`, which the `.flowise` line collects; no row names
them as a project.

#### Relocation variables

| Variable | Moves | Replaces | Table agent |
| --- | --- | --- | --- |
| `DATABASE_PATH` | the directory holding `database.sqlite`, with the default SQLite database ([DataSource.ts:23](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/DataSource.ts#L23)) | `.flowise` | `flowise` |
| `SECRETKEY_PATH` | the directory holding `encryption.key` and the auth signing secrets ([utils/index.ts:1670](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/index.ts#L1670), [1584-1585](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/index.ts#L1584-L1585)) | `.flowise` | `flowise` |
| `BLOB_STORAGE_PATH` | the chat storage directory, and `uploads` beneath it ([storageUtils.ts:51-53](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/src/storageUtils.ts#L51-L53), [utils/index.ts:2019-2021](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/index.ts#L2019-L2021)) | `.flowise/storage`, `.flowise/uploads` | `flowise` |
| `LOG_PATH` | the log directory ([utils/config.ts:10](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/config.ts#L10)) | | `flowise` |

Not relocation variables: `E2B_APIKEY` (carries a key), `DATABASE_TYPE` (behaviour only),
`HOME` and `USERPROFILE` (generic).

#### Docker

The README compose bind-mounts the host's `~/.flowise` to
`/home/node/.flowise`
([docker/docker-compose.yml:162-165](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/docker/docker-compose.yml#L162-L165)),
so the home line already covers it. The only named volume is `redis_data`,
in the queue compose
([docker-compose-queue-prebuilt.yml:9-10](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/docker/docker-compose-queue-prebuilt.yml#L9-L10),
[340-342](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/docker/docker-compose-queue-prebuilt.yml#L340-L342));
it is generic and not matched. An inferred `*flowise*` volume line, like
the inferred block for Tabby and others, was considered and not added:
the documented deployment uses a bind mount.

#### Database detail

The records inside `database.sqlite`, at
[`9291856`](https://github.com/FlowiseAI/Flowise/commit/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb):

- **Per-call decisions.** A Human Input node (`humanInputAgentflow`)
  stops the run and stores the request as `chat_message.action`, JSON
  with an `id`, a `mapping` of `approve: "Proceed"` and
  `reject: "Reject"`, and the node's id, label and input
  ([buildAgentflow.ts:1409-1425](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/buildAgentflow.ts#L1409-L1425),
  [2406-2407](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/buildAgentflow.ts#L2406-L2407),
  [ChatMessage.ts:49-50](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/database/entities/ChatMessage.ts#L49-L50)).
  The answer is a `humanInput` of `type` `proceed` or `reject`, a
  `startNodeId` and optional `feedback`
  ([components Interface.ts:492-496](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/src/Interface.ts#L492-L496)).
  On resume the stopped step is dropped from the execution data and the
  node's new output records `conditions` with `isFulfilled` on the chosen
  outcome and the feedback or the type as `content`
  ([buildAgentflow.ts:1195-1202](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/buildAgentflow.ts#L1195-L1202),
  [HumanInput.ts:155-190](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/nodes/agentflow/HumanInput/HumanInput.ts#L155-L190)).
  The user message saved for the resume is the feedback, or a single
  space when there was none, so `chat_message` alone does not say which
  button was pressed
  ([buildAgentflow.ts:2345-2365](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/utils/buildAgentflow.ts#L2345-L2365)).
  An Agent node whose tool has `agentSelectedToolRequiresHumanInput` set
  stops before the call with the call's JSON in its output and
  `isWaitingForHumanInput`; `reject` removes the tool, `proceed` runs it
  ([Agent.ts:266](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/nodes/agentflow/Agent/Agent.ts#L266),
  [2241-2268](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/nodes/agentflow/Agent/Agent.ts#L2241-L2268),
  [2631-2642](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/nodes/agentflow/Agent/Agent.ts#L2631-L2642)).
  No standing allow list exists apart from the MCP custom-script allow
  list cited in [Source and evidence](#source-and-evidence), which is
  server configuration.
- **Model.** An Agent node's step in `execution.executionData` keeps the
  node's inputs, among them `agentModel` and `agentModelConfig`, whose
  `model` or `modelName` is the model id
  ([Agent.ts:695-700](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/nodes/agentflow/Agent/Agent.ts#L695-L700),
  [1506-1514](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/nodes/agentflow/Agent/Agent.ts#L1506-L1514)).
  `chat_message` has no model column
  ([ChatMessage.ts:8-66](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/database/entities/ChatMessage.ts#L8-L66)),
  so a chatflow's model is otherwise only in the node inputs of its
  canvas JSON, `chat_flow.flowData`.
- **Executions.** `execution` (`executionData` JSON, `state` one of
  `INPROGRESS`, `FINISHED`, `ERROR`, `TERMINATED`, `TIMEOUT`, `STOPPED`,
  `agentflowId`, `sessionId`, `action`, `createdDate`, `stoppedDate`)
  holds every Agentflow v2 run step by step
  ([Execution.ts:7-46](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/database/entities/Execution.ts#L7-L46),
  [server Interface.ts:23](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/Interface.ts#L23)).
  `chat_message.executionId` joins it to the chat. Beside `chat_message`, the primary store,
  `execution`, `chat_message_feedback`, `upsert_history` and the two schedule tables below
  record past use.
- **Schedules.** `schedule_record` (`triggerType` `AGENTFLOW`,
  `targetId`, `cronExpression`, `timezone`, `enabled`) and
  `schedule_trigger_log` (`scheduleRecordId`, `executionId`, `status` one
  of `QUEUED`, `RUNNING`, `SUCCEEDED`, `FAILED`, `SKIPPED`,
  `scheduledAt`) record scheduled Agentflow runs, which have no user
  message of their own
  ([ScheduleRecord.ts:5-37](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/database/entities/ScheduleRecord.ts#L5-L37),
  [ScheduleTriggerLog.ts:6-51](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/database/entities/ScheduleTriggerLog.ts#L6-L51)).
- **Third-party tracing.** `chat_flow.analytic` names the tracing
  services a chatflow reports to (LangSmith, Langfuse, Lunary, LangWatch,
  Arize, Phoenix, Opik), and tracing environment variables add more
  ([ChatFlow.ts:39](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/database/entities/ChatFlow.ts#L39),
  [handler.ts:518-532](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/components/src/handler.ts#L518-L532)).
  The runs are then also in that service's account; no local row records
  the remote run id.
- **Deletion.** Clearing a chat removes its `chat_message_feedback` rows,
  its upload directory and its `chat_message` rows, and leaves its
  `execution` rows
  ([chat-messages/index.ts:110-135](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/services/chat-messages/index.ts#L110-L135));
  deleting messages by id also deletes the executions they name
  ([144-180](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/services/chat-messages/index.ts#L144-L180)).
  Deleting a chatflow removes its messages, feedback and upsert history
  ([chatflows/index.ts:135-147](https://github.com/FlowiseAI/Flowise/blob/9291856d1ea4a4ceea9f8fef8ce14f4f6c81e8eb/packages/server/src/services/chatflows/index.ts#L135-L147)).

### Credentials

- `.flowise/encryption.key` and the four `*_secret.key` files: flagged.
- `database.sqlite` holds encrypted credentials and the chat history
  together, so it is collected unflagged; with `encryption.key` the
  `credential` rows can be decrypted.

### Exclusions

`.flowise/vectorstore`.

### Project-local files

None. Flows are rows in `database.sqlite`.

### Where the project path is recorded

Nowhere; Flowise has no project concept.

## Parsing

Flowise is `backlog`: its parser is
[issue 68](https://github.com/seanthegeek/doubleagent/issues/68), and nothing it writes is parsed
yet. Everything a parser would read is in `.flowise/database.sqlite`
([Database detail](#database-detail)); there is no line-oriented file under the home, and the
logs are outside it ([Per-user storage](#per-user-storage)).

## Confidence

- High for every path and table (source). Nothing material was left undetermined.
