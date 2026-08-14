## ADDED Requirements

### Requirement: PromptStore 线程安全地持有并原子更新 prompt 内容
`PromptStore` SHALL 使用 `atomic.Pointer` 持有一个 `promptSnapshot` 结构体指针。
`promptSnapshot` 包含 `SystemPrompt string`、`SystemPromptMD5 string`、`Instruction string`、`InstructionMD5 string` 四个字段。

`PromptStore` 对外暴露的方法：
- `SystemPrompt() string`：返回当前 system prompt 内容（无锁读）
- `Instruction() string`：返回当前 instruction 内容（无锁读）
- `SystemPromptMD5() string`：返回当前 system prompt MD5
- `InstructionMD5() string`：返回当前 instruction MD5
- `Update(sp, spMD5, inst, instMD5 string)`：原子替换快照

#### Scenario: 并发读取 prompt 内容
- **WHEN** 多个 goroutine 同时调用 `SystemPrompt()` 或 `Instruction()`
- **THEN** 每次调用均返回一致的值，不发生 data race

#### Scenario: 更新 prompt 内容后立即可见
- **WHEN** 调用 `Update()` 更新 system prompt 内容
- **THEN** 下一次调用 `SystemPrompt()` 立即返回新内容

### Requirement: PromptStore 支持初始内容注入
`PromptStore` 的构造函数 `NewPromptStore(sp, spMD5, inst, instMD5 string) *PromptStore` SHALL 接受初始内容，
使得从文件或首次远程同步中获取的内容可以作为初始值。

#### Scenario: 进程启动时从文件加载初始 prompt
- **WHEN** BKAIDev 同步未启用，传入文件内容构造 PromptStore
- **THEN** PromptStore 返回文件内容，MD5 为空字符串
