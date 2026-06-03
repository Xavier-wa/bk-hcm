## Context

Agent-server 已完成 skill 的 BKAIDev 托管：BKAIDev 提供 skill 的列表接口和下载接口，agent-server 通过 `manifest.Store` 进行版本追踪（以 `version` 字段为版本号），并将 skill 文件安装到本地目录，由 `FSRepository` 对外提供服务。

Prompt（system prompt / instruction）目前通过本地文件路径配置，在进程启动时一次性读入内存。BKAIDev 的 prompt 接口不提供版本号字段，但提供 content 内容，可通过对 content 计算 MD5 来实现版本识别。

与 skill 的关键区别：
- Skill 是"多个"（N 个文件，每个有独立目录），prompt 是"有限个已知条目"（仅 system prompt 和 instruction）
- Skill 通过文件系统和 `FSRepository` 提供服务，prompt 是纯内存字符串，通过 BeforeModel callback 在每次 LLM 调用前注入
- Skill 通过 list 接口全量拉取后 diff，prompt 直接按 prompt_id 调用 retrieve 接口即可，无需 list

## Goals / Non-Goals

**Goals:**
- 通过 `prompt_id` 从 BKAIDev 获取 system prompt 和 instruction 内容，替代本地文件读取
- 通过 content MD5 追踪版本，manifest 持久化到本地文件，仅在内容变更时触发更新
- 首次同步完成后才将 `promptReady` 置为 true，与 skill readiness 机制对称
- 通过定时 cron 任务（与 skill sync 相同的定时框架）实现定期增量同步
- 通过 `BeforeModel` callback 在每次 LLM 调用前用最新内容替换系统消息，支持热更新
- 为 GraphAgent 和 LLMAgent 均实现 prompt 替换，保持一致行为

**Non-Goals:**
- 不实现 prompt 的列表展示或用户选择（固定由配置的 prompt_id 驱动）
- 不支持 prompt 变量（variables）的动态替换，仅使用 content 原文
- 不变更现有的文件模式配置（`systemPromptFile` / `instructionFile`），保持向后兼容
- 不引入数据库存储，纯文件 manifest + 内存持有

## Decisions

### 1. 使用 `retrieve_app_v1_prompts`（按 ID 获取）而非 `list_app_v1_prompts`

**理由**：Prompt 配置是有限的已知条目（最多 2 个：system prompt 和 instruction），用 prompt_id 直接 retrieve 更简单且无歧义。`list_app_v1_prompts` 需要分页、过滤，对于固定 ID 的场景属于过度设计。

**替代方案**：使用 list 接口按 prompt_code 过滤 → 复杂度高，且需要额外的字段映射。

### 2. Content MD5 作为版本标识

**理由**：BKAIDev prompt 接口无版本号字段，`updated_at` 虽然存在但精度不稳定，content MD5 是最可靠的内容变更检测手段。`manifest.Entry` 已有 `MD5` 字段，可直接复用。

**替代方案**：使用 `updated_at` 时间戳 → 依赖服务端时钟，存在精度问题。

### 3. 引入独立的 `PromptStore` 而非复用 cc.AgentPromptConfig

**理由**：`cc.AgentPromptConfig` 在进程启动时初始化，是只读值；热更新需要一个支持并发读写的可变容器。`PromptStore` 使用 `sync/atomic` 存储 `*promptSnapshot`（包含 content 和 MD5），实现无锁读路径，写路径原子替换。

**替代方案**：使用 `sync.RWMutex` → 可行，但 atomic pointer swap 更简洁。

### 4. PromptReplaceCallback 替换 msgs[0] 的系统消息

**理由**：GraphAgent 的 system prompt 在 `BuildGraph` 时作为静态字符串传入，之后无法修改；LLMAgent 的 `WithGlobalInstruction` 同理。唯一的运行时注入点是 `BeforeModel` callback，它在每次 LLM 调用前执行，可直接修改 `args.Request.Messages`。

**实现**：
```
msgs[0] = System{ Content: sp.Content() }  // 替换为 PromptStore 最新值
```
仅当 `msgs[0].Role == RoleSystem` 时替换（不新增），其余 messages 不变。

**替代方案**：通过 channel 通知重建 agent → 需要重建整个 runner，代价过高。

### 5. Syncer 简化实现（不复用 skill.Syncer 结构）

**理由**：Prompt sync 逻辑远比 skill 简单（无并行安装、无目录管理、无 list diff），强行复用 `skill.Syncer` 会引入不必要的抽象。新建 `cmd/agent-server/logics/prompt/` 包，包含 `Syncer`、`Store`、callback 三个文件，各自职责清晰。

Cron 任务注册模式与 skill 保持完全一致（实现 `core.Task` 接口，通过 `cron.Register` 注册）。

### 6. 配置扩展：AgentPromptConfig 新增 BKAIDev 子块

```yaml
prompt:
  bkaidev:
    enabled: true
    # 继承 skills.bkaidev 的 apiGateway 配置（或单独配置）
    apiGateway: ...
    spaceID: "system-xxx"
    syncInterval: "5m"
    systemPromptID: 411     # retrieve_app_v1_prompts 的 prompt_id
    instructionID: 125      # 0 表示不配置
    manifestPath: "/data/agent/prompt-manifest.json"
```

文件模式（`systemPromptFile` / `instructionFile`）与 BKAIDev 模式互斥：当 `bkaidev.enabled=true` 时，`trySetDefault` 跳过文件加载；BKAIDev 未启用时维持现有行为。

## Risks / Trade-offs

- **[风险] BKAIDev 接口不可用时 prompt 无法更新** → 缓解：manifest 持久化到本地文件，进程重启后直接使用上次成功同步的内容；首次同步失败时 `promptReady` 不就绪，服务不对外暴露，避免 prompt 为空的情况
- **[风险] MD5 冲突** → MD5 碰撞概率极低，对于 prompt 内容（通常几 KB）实际不存在风险
- **[Trade-off] BeforeModel callback 每次调用都替换系统消息** → 绝大多数情况下 content 不变，替换操作只是一次字符串赋值，性能影响可忽略；可通过 MD5 比较跳过无变化的替换（PromptStore 暴露版本快照，callback 本地缓存上次版本）

## Migration Plan

1. 配置不变，`bkaidev.enabled=false` 时行为与现在完全一致，无 breaking change
2. 启用 BKAIDev prompt 同步时：
   - 在配置文件中添加 `prompt.bkaidev` 块，设置 `enabled: true`，配置 `systemPromptID` / `instructionID`
   - 移除或保留 `systemPromptFile` / `instructionFile`（BKAIDev 模式下被忽略）
   - 首次启动时服务处于 not-ready 状态直到首次同步完成
3. 回滚：将 `enabled` 改回 `false`，恢复文件路径配置，重启服务

## Open Questions

- 无。配置路径、同步逻辑、readiness 机制均有明确对照（参照 skill 实现）。
