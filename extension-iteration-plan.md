# Extension 迭代实施方案

## 目标

基于 `prd-extension.md`，结合当前仓库真实实现，给出一个可执行的迭代方案。目标不是直接照搬 PRD 的理想终态，而是在尽量少破坏现有主链路的前提下，把 extension 能力逐步落到当前 Go CLI/TUI 架构中。

## 现状校准（as-is）

先对当前仓库做现实对齐，避免后续阶段拆解偏离真实代码树。

### 已存在代码

- 项目是 Go 实现的 AI Coding CLI/TUI，不是浏览器插件工程，见 `README.md:3`、`docs/architecture.md:15`。
- 扩展层目前已存在最小占位实现：`internal/extensions/manager.go:9` 定义了 `ExtensionKind`、`ExtensionInfo`、`Manager` 与 `NopManager`，但还没有真正的生命周期、manifest、resolver、错误码体系。
- tools registry 已存在且被广泛依赖，当前入口是 `internal/tools/registry.go:47` 的 `DefaultRegistry()` 与 `internal/tools/registry.go:62` 的 `Add`。
- policy 包并非未来目标目录，而是已存在实现；当前工具权限判定入口是 `internal/policy/access.go:21`，skill policy 到 allow/deny 集合的映射在 `internal/policy/tools.go:13`。
- skills 能力已具备目录扫描、覆盖、解析与查找，见 `internal/skills/manager.go:17`、`internal/skills/manager.go:49`。
- executor 主链路已稳定，工具名直接参与权限判定，见 `internal/tools/executor.go:123`。
- runner 侧当前通过 `ExecutionContext.AllowedTools/DeniedTools` 传递策略结果，而不是调用独立 extension policy bridge，见 `internal/agent/tool_execution.go:48`。
- CLI 顶层命令分发并不在 `cmd/bytemind/main.go` 内扩展，而是由 `internal/app/cli_dispatch.go:15` 控制；当前只支持 `chat`、`tui`、`run`、`install`、`help`。
- `prd-extension.md` 在仓库中真实存在，可作为目标参照，路径为 `prd-extension.md`。

### 目标目录 vs 已有目录

为避免混淆，本文后续统一按下面约定表述：

- **已有目录/文件**：当前仓库中已经存在并可直接演进的实现，例如 `internal/tools`、`internal/extensions`、`internal/skills`、`internal/policy`、`internal/app`。
- **目标目录/文件**：为承接 extension 迭代而建议新增的代码位置，例如 `internal/extensions/bridge.go`、`internal/extensions/types.go`、`internal/extensions/mcp/*`。
- 若某阶段写到“新增 package / 新增文件”，默认含义是**在已有代码树上增量落地**，不是假设仓库已经有对应模块。

当前项目事实基线：

- 当前 extension 模块仍处于占位阶段，只有最小 `NopManager` 与基础类型，见 `internal/extensions/manager.go:9`、`internal/extensions/manager.go:38`。
- tools registry 仍然采用内存 map + `panic` 注册语义，尚未满足 PRD 要求的并发安全和结构化错误契约，见 `internal/tools/registry.go:43`、`internal/tools/registry.go:62`。
- skills 已具备发现、覆盖、解析能力，可作为 extension 统一接入的第一批复用基础，见 `internal/skills/manager.go:17`、`internal/skills/manager.go:49`。
- executor 主链路已经比较稳定，应保持“统一执行、不感知 extension 特判”的原则，和 PRD 一致，见 `internal/tools/executor.go:42`、`prd-extension.md:519`。

## 当前差距判断

### 已有能力

1. 已有 tools 抽象、registry、executor 主链路。
2. 已有 skills 发现与解析能力。
3. 已有 agent runner 作为工具调用总入口。
4. 已有基础测试体系，适合按 package 渐进补强。

### 主要缺口

1. extension 领域模型不完整，当前只有 `ExtensionKind`、`ExtensionInfo`、`Manager` 的简化版本，缺少状态、manifest、错误码、resolver 等核心契约，见 `internal/extensions/manager.go:9`。
2. registry 无并发保护，`Add` 失败直接 `panic`，不适合作为扩展接入底座，见 `internal/tools/registry.go:62`。
3. extension 生命周期尚未形成真正状态机，当前也没有 loaded/active/degraded/stopped 的迁移约束。
4. extension 到 tools/policy 的 source-aware 命名桥尚不存在。
5. skills 还没有作为 extension 被统一封装，MCP adapter 也尚未落地。
6. 目前缺少 extension 级 contract test、并发 race 测试、故障隔离回归测试。

## 迭代原则

1. **先底座，后接入**：先改 registry 契约，再做 extension 统一建模，否则后续接 MCP/skills 会反复返工。
2. **先 skill，后 MCP**：当前仓库已有 `internal/skills` 可复用，skill adapter 是最稳妥的首个真实 extension 来源。
3. **不侵入 executor**：继续让 `internal/tools/executor.go` 保持统一执行入口，不引入 extension 专用分支。
4. **对 runner 低感知**：runner 只消费最终工具定义与执行结果，不管理 extension 内部状态。
5. **所有失败局部化**：单扩展加载失败、桥接失败、健康检查失败，只影响该扩展，不影响 builtin tools。
6. **先内存态，后持久化**：本轮只做内存状态与事件接口，不扩展审计落盘。

## 推荐迭代顺序

---

## 阶段 1：Registry 底座改造

### 目标

让 `internal/tools/registry.go` 成为 extension 接入的可靠底座，满足 PRD 的前置要求。

### 为什么先做

因为当前 registry 的几个问题会直接阻塞后续所有工作：

- `Add` 直接 `panic`，扩展加载失败会导致进程级风险，见 `internal/tools/registry.go:72`。
- 内部 `map[string]ResolvedTool` 无锁，不满足并发 load/unload/list/get 场景，见 `internal/tools/registry.go:43`。
- 无 source/extension 维度，无法表达冲突来源。

### 具体改造

#### 1.0 先做兼容迁移，不直接切断旧 API

当前 `Add` 已被 `DefaultRegistry()`、`internal/tools/registry_test.go` 以及 runner 相关测试广泛直接使用，见 `internal/tools/registry.go:47`、`internal/tools/registry.go:62`。因此第一步不建议“一次性切到 Register/Unregister 并让旧调用全部失效”，而应采用分层迁移：

- 第一步：新增 `Register(tool Tool, opts RegisterOptions) error`、`Unregister(name string) error`、`Get(name string) (ResolvedTool, bool)`、`List() []ResolvedTool`。
- 第二步：保留 `Add(tool Tool)` 作为兼容入口，但内部改为调用 `Register(..., builtin opts)`；仅在 builtin 初始化路径上保留 `mustRegisterBuiltin(...)` 这类私有 fail-fast helper。
- 第三步：先让扩展接入与新增代码全部使用 `Register/Unregister`，旧测试与旧调用逐步迁移。
- 第四步：待 DefaultRegistry、主要测试和调用点全部切换后，再评估是否正式弃用 `Add`。

建议把兼容窗口明确写进实现说明：**本轮迭代以“新增 API + 旧 API 兼容包裹”为目标，不做破坏式删除**。

#### 1.1 重构 Registry 数据结构

建议把 `Registry` 从：

- `tools map[string]ResolvedTool`

改为至少包含：

- `mu sync.RWMutex`
- `tools map[string]ResolvedTool`
- `meta map[string]RegistrationMeta`（可选，但强烈建议）

其中 `RegistrationMeta` 至少包含：

- `ToolKey`
- `Source`
- `ExtensionID`
- `ConflictPolicy`

这样后续 bridge 才能向 policy 和错误语义透传上下文。

#### 1.2 注册时做防御性复制

仅加锁还不够。当前 `ResolvedTool.Definition.Function.Parameters` 是 `map[string]any`，若继续把外部传入的 definition/spec 直接存入 registry，后续并发读写或调用方复用同一个 definition 时，仍可能出现共享引用与 race。

因此建议：

- `Register` 时对 `llm.ToolDefinition` 做深拷贝，至少深拷贝 `Function.Parameters`。
- 若 `ToolSpec` 中也包含 map/slice，可一并做复制。
- `Get/List/Definitions*` 返回值基于快照复制，避免向外暴露 registry 内部可变对象。

这样 `-race` 才更有机会真实兜住问题，而不是只把 map 容器本身加锁。

#### 1.3 将 Add 改为返回结构化错误

建议保留兼容层，但新增主入口：

- `Register(tool Tool, opts RegisterOptions) error`
- `Unregister(name string) error`
- `Get(name string) (ResolvedTool, bool)`
- `List() []ResolvedTool`

兼容方案：

- `DefaultRegistry()` 内部改用 `mustRegisterBuiltin(...)` 之类私有 helper；builtin 工具初始化失败仍可在启动期暴露，但不让 extension 走 `panic` 路径。

#### 1.4 明确冲突策略

默认实现直接采用 PRD 要求的 `reject`：

- builtin vs builtin：启动即失败
- builtin vs extension：拒绝 extension 注册
- extension vs extension：拒绝后注册者

错误对象建议包含：

- `code`
- `message`
- `tool_key`
- `source`
- `extension_id`
- `conflict_with`
- `cause`

#### 1.5 快照语义

`Definitions*`、`ResolveForModeWithFilters`、未来的 `List/Get` 都要在读锁下复制快照，再在锁外排序与过滤，避免暴露部分写入态。

### 交付物

- 修改 `internal/tools/registry.go`
- 补强 `internal/tools/registry_test.go`
- 新增 `internal/tools/registry_contract_test.go`
- 可选新增 `internal/tools/registry_errors.go`

### 测试建议

先跑窄测试：

- `go test ./internal/tools -run Registry -v`
- `go test ./internal/tools -race`

### 完成标志

1. 注册失败不再 `panic`。
2. 并发 register/unregister/get/list 没有 race。
3. 冲突错误能定位来源。

---

## 阶段 2：建立 extensions 统一域模型

### 目标

把当前占位式 `internal/extensions/manager.go` 拆成可演进的统一模型层。

### 当前问题

当前文件把类型和 manager 接口塞在一起，而且状态模型不完整，见 `internal/extensions/manager.go:9`、`internal/extensions/manager.go:24`。

### 具体改造

建议新增以下文件：

- `internal/extensions/types.go`
- `internal/extensions/manifest.go`
- `internal/extensions/interface.go`
- `internal/extensions/errors.go`

建议先沉淀这些核心类型：

- `ExtensionKind`
- `ExtensionStatus`
- `ErrorCode`
- `Manifest`
- `Capability`
- `ExtensionInfo`
- `ExtensionTool`
- `ExtensionEvent`
- `ActivateOptions`
- `HealthSnapshot`

建议接口拆分为：

- `Extension`
- `Resolver`
- `Manager`
- `EventSink` 或 `Reporter`

### 设计细节

#### 2.1 保持向后兼容

当前 `tools.ExecutionContext` 里已经依赖 `extensionspkg.Manager`，见 `internal/tools/registry.go:24`。因此新 `Manager` 接口尽量维持原方法集并向上兼容：

- `Load`
- `Unload`
- `List`
- `Get`

原 `NopManager` 可以继续保留，但放到更清晰的 `nop.go` 或保留在 manager 文件中。

#### 2.2 事件模型先最小实现

先只定义事件结构，不急着接 storage：

- `load`
- `activate`
- `degraded`
- `recover`
- `unload`

与 PRD 的最小集合保持一致。

### 交付物

- 新增 `internal/extensions/types.go`
- 新增 `internal/extensions/interface.go`
- 新增 `internal/extensions/errors.go`
- 适度瘦身 `internal/extensions/manager.go`

### 测试建议

- `go test ./internal/extensions -run 'Type|Status|Manager' -v`

### 完成标志

1. extension 领域模型独立成层。
2. executor 和 runner 侧无需额外改造即可继续编译通过。

---

## 阶段 3：实现发现与校验管线

### 目标

先把“可发现、可校验、可拒绝非法输入”的管线建起来，但第一轮建议只覆盖 **local/workspace skill extension**，MCP 先不接入真实网络行为。

### 为什么这样做

当前 `internal/skills/manager.go` 已经包含多目录扫描和覆盖规则，见 `internal/skills/manager.go:53`、`internal/skills/manager.go:66`。先复用现有目录布局，可以最小成本证明 extension 管线成立。

### 具体改造

建议新增：

- `internal/extensions/discovery.go`
- `internal/extensions/validator.go`

第一轮 source 类型建议只支持：

- `builtin-skill`
- `user-skill`
- `project-skill`
- `local-extension-dir`

先不要引入过重的 JSON Schema 引擎依赖；可以先用 Go 结构体验证 + 版本字段检查。若仓库已有稳定 schema 依赖再补 JSON Schema。

### 关键方案

#### 3.1 发现层只产出引用，不直接实例化

建议：

- `Discover(...) -> []ManifestRef`
- `Validate(ref) -> Manifest`

这样后面 lifecycle manager 能把“发现/校验/实例化/激活”拆开，便于隔离失败。

#### 3.2 skills 复用策略

对已有 skills，不要求立刻迁移目录结构；可以通过 adapter 把 `internal/skills` 的 `Skill` 映射成 extension manifest。

也就是说，发现层不一定只扫描新的 `extensions/` 目录，可以将现有 skill catalog 作为一个逻辑 source。

### 交付物

- 新增 `internal/extensions/discovery.go`
- 新增 `internal/extensions/validator.go`
- 如确有必要，再新增 `internal/extensions/schema/manifest.schema.json`

### 测试建议

- 多源扫描顺序
- 去重规则
- 非法 manifest 拒绝
- 版本不兼容拦截

命令可先跑：

- `go test ./internal/extensions -run 'Discover|Validate' -v`

### 完成标志

1. 能从现有 skill 来源构建标准化 manifest 引用。
2. 非法输入不会进入加载流程。

---

## 阶段 4：实现生命周期管理器

### 目标

从当前 `NopManager` 升级为真正可用的内存态 manager，管理 loaded/active/degraded/stopped。

### 当前问题

当前 manager 只是空实现，无法承接 PRD 中 load/unload/list/get 的真实语义，见 `internal/extensions/manager.go:38`。

### 具体改造

建议新增：

- `internal/extensions/lifecycle.go`
- `internal/extensions/state_store.go`

并把 `manager.go` 变成真正的 manager 实现入口。

### 状态机建议

#### 合法状态迁移

- `loaded -> active`
- `active -> degraded`
- `degraded -> active`
- `active -> stopped`
- `loaded -> stopped`
- `degraded -> stopped`

#### 非法状态迁移

- `stopped -> degraded`
- `stopped -> active`（除非重新 load）
- `loaded -> degraded`

### 并发策略

- manager 内部单独维护 `stateStore`
- `List/Get` 返回复制后的快照
- `Load/Unload/Activate/Deactivate` 在单扩展粒度上串行化

这里建议避免一开始就做复杂 actor 模型，当前仓库风格更接近 `RWMutex + 明确状态迁移函数`。

### 交付物

- 实现真实 manager
- state store 内存实现
- 状态迁移校验
- 最小事件上报接口

### 测试建议

- 并发重复加载返回 `already_loaded`
- 非法迁移被拒绝
- `List/Get` 快照一致

命令可先跑：

- `go test ./internal/extensions -run 'Manager|Lifecycle' -v`
- `go test ./internal/extensions -race`

### 完成标志

1. manager 可真实管理 extension 状态。
2. 单扩展异常不会影响其他扩展记录。

---

## 阶段 5：优先落地 Skills Adapter

### 目标

把 legacy skills 先统一纳入 extension 系统，这是最适合当前仓库的第一批真实流量。

### 为什么先做 skill adapter

- 当前已有 `internal/skills/manager.go` 可直接复用。
- skill 是本地资源，调试简单，不涉及 MCP 握手、网络波动、协议兼容。
- 可以先验证 extension -> bridge -> tools/policy 全链路。

### 具体改造

建议新增：

- `internal/extensions/skills/adapter.go`
- `internal/extensions/skills/compat.go`
- `internal/extensions/skills/cache.go`

### 设计细节

#### 5.1 兼容旧 skill 结构

`internal/skills` 里的 skill 包含目录、frontmatter、manifest、tool policy 等信息，见 `internal/skills/manager.go:158`。adapter 需要补齐 extension 侧缺失字段：

- `ID`
- `Kind=skill`
- `Entry`
- `Capabilities`
- `UpdatedAt`

#### 5.2 热路径先补底层快照接口，再谈 adapter 缓存

PRD 明确要求不要在 `List/Find` 热路径做全量 reload。当前 `skills.Manager.List()` 与 `Find()` 的确都会命中 `Reload()`，见 `internal/skills/manager.go:120`、`internal/skills/manager.go:145`。因此如果只在 adapter 层加缓存，仍可能被底层 reload 抵消。

建议顺序调整为：

- 先在 `internal/skills/manager.go` 增加**只读快照 API**，例如 `Snapshot()` / `Catalog()` 之类，只返回当前已加载数据，不触发 reload。
- 再把刷新时机外提到 extension lifecycle：由显式 `Reload`、load/unload、配置变更或启动阶段触发，而不是由查询热路径触发。
- adapter 缓存只作为上层增量包装，不再承担“拦住底层强制 reload”的职责。

这样才能真正满足 extension manager 的热路径要求。

#### 5.3 缓存策略

在只读快照 API 落地后，adapter 层可再加一层轻缓存：

- key：source + skill dir + modtime/hash
- 失效条件：显式 reload、目录变化、配置变化、unload

第一版不必做文件系统 watcher，先做拉模式增量刷新即可。

#### 5.4 工具语义

skill adapter 第一轮不一定要把 skill 本身变成“可执行工具”；更合理的是：

- 先把 skill 作为 extension 元数据接入和管理对象
- 再在 bridge 阶段决定哪些 skill 能投影成 tool 或策略能力

如果当前业务要求 skills 最终映射成 tools，则要明确哪些字段来自 `skill.json`/`SKILL.md`，避免把 prompt-only skill 误注册成 tool。

### 交付物

- skill -> extension 映射器
- 缓存与增量刷新逻辑
- 对旧 skill 兼容不破坏现有 runner 行为

### 测试建议

- 旧 skill 直接可加载
- 缓存命中不重复 reload
- skill 更新后能刷新

命令可先跑：

- `go test ./internal/extensions/... -run Skill -v`
- `go test ./internal/skills -v`

### 完成标志

1. 现有 skills 可以通过 extension manager 被发现和列出。
2. 热路径不依赖全量 `Reload()`。

---

## 阶段 6：实现 Bridge，接入 tools 与 policy 语义

### 目标

把 extension 能力稳定地注册到 tools registry，同时做到 source-aware 命名，避免与 builtin 工具冲突。

### 当前问题

当前 `tools.Registry` 只按 `definition.Function.Name` 存储，无法表达 `source:extensionID:toolName` 这类稳定键，见 `internal/tools/registry.go:75`。

### 具体改造

建议新增：

- `internal/extensions/bridge.go`
- `internal/extensions/bridge_test.go`

### 核心方案

#### 6.1 先定义 stable key 约束，再定最终格式

当前工具名会直接透传到执行与权限判定链路，见 `internal/tools/executor.go:123`、`internal/agent/tool_execution.go:48`。因此 stable key 不能只从“内部可读性”出发设计，必须先验证它是否兼容模型/provider 对 tool name 的限制。

建议先补一个独立前置设计：

- 定义“可跨 provider 的 key 字符集”与长度约束。
- 新增 contract test，覆盖 builtin 名称、extension 名称、含分隔符名称、超长名称、大小写与归一化规则。
- 先把生成器收敛为可替换实现，再决定最终是否使用 `source:extensionID:toolName`、`source__extensionID__toolName` 或其他编码方式。

在约束与测试没落地前，不建议把 `:` 直接写死为最终分隔符。

#### 6.2 双名称模型

建议区分：

- **display name**：给模型或界面展示的名字
- **stable key**：内部注册和 policy 判断使用的名字

这里需要谨慎，因为当前 executor 和 policy 都是按 tool name 工作，见 `internal/tools/executor.go:123`。为了尽量少改现有链路，第一版可以继续让 registry 使用稳定键，但前提是 stable key 已通过 provider 约束测试。

如果后续发现 provider 约束更严格，则需要保留 display/stable 双轨并在 runner 层增加映射，而不是把分隔格式硬编码进所有下游。

#### 6.3 builtin 兼容性

builtin 继续保留原名：

- `read_file`
- `write_file`

extension 一律稳定键化，但具体格式以后续 contract test 结论为准，例如：

- `skill:repo-onboarding:open_doc`
- `mcp:github:list_prs`

目标是让 policy 与 executor 不需要再做二次映射，但前提是名称格式已验证可跨 provider 使用。

#### 6.4 registry 中保留来源元信息

bridge 注册时必须同步登记：

- source
- extension id
- original tool name
- stable key

否则排查冲突时定位信息不够。

#### 6.5 policy 集成按现有 allow/deny 路径迁移

当前并不是“独立 policy bridge 决定工具执行”，而是：

- skill policy 先在 `internal/policy/tools.go:13` 映射成 allow/deny 集合；
- runner 再把集合写入 `tools.ExecutionContext`，见 `internal/agent/tool_execution.go:48`；
- executor 最终按 `resolved.Definition.Function.Name` 做判定，见 `internal/tools/executor.go:123`。

因此本阶段不应抽象成脱离现状的“bridge/policy stable key 打通”，而应按下面顺序迁移：

1. 先让 extension tool 注册后的最终名称与 allow/deny 集合使用同一稳定键。
2. 再让 skill/extension policy 产出的条目逐步迁移到稳定键集合。
3. 在兼容期内允许 builtin 继续使用原名，避免一次性打断现有 skill policy。
4. 为旧 skill policy 名称增加兼容映射或告警，直到策略定义也完成迁移。

### 交付物

- `Bridge(extensionTool) tools.Tool`
- stable key 生成器
- 冲突拒绝与错误透传

### 测试建议

- builtin 与 extension 同名时拒绝注册
- extension 间同名时也拒绝
- policy 使用稳定键时权限判定正确

命令可先跑：

- `go test ./internal/extensions -run Bridge -v`
- `go test ./internal/tools -run Registry -v`
- `go test ./internal/policy -v`

### 完成标志

1. extension tool 能进入 registry。
2. policy 层不会被同名 builtin 污染。

---

## 阶段 7：补 MCP Adapter

### 目标

在 skill adapter 验证完成后，再接入 MCP，降低系统性风险。

### 具体改造

建议新增：

- `internal/extensions/mcp/adapter.go`
- `internal/extensions/mcp/client.go`
- `internal/extensions/mcp/health.go`

### 实施建议

第一轮只做最小可用：

1. 建立 MCP server 配置结构。
2. 完成握手与能力发现。
3. 把 MCP 能力映射成 `ExtensionTool`。
4. 握手失败进入 `degraded`，不阻塞其他扩展。

### 注意点

- 不要把 MCP 特判塞进 executor。
- client 错误需统一映射到 extension 错误码。
- 单个 MCP tool 声明不合法时，优先跳过单工具，不拖垮整个 server。

### 测试建议

- fake MCP server 握手成功/失败
- 工具能力缺字段时跳过
- degraded 状态可见

### 完成标志

1. MCP 能力可以被 manager 加载并桥接。
2. 握手失败只影响单扩展。

---

## 阶段 8：故障隔离与恢复

### 目标

把 extension 从“能用”提升到“可运行、可降级、可恢复”。

### 具体改造

建议新增：

- `internal/extensions/isolation.go`
- `internal/extensions/recovery.go`
- `internal/extensions/health_manager.go`

### 建议策略

#### 8.1 健康状态独立维护

每个 extension 单独维护：

- failure count
- last failure
- circuit state
- next retry time

#### 8.2 熔断策略

推荐先实现简单版本：

- 连续失败达到阈值 -> `degraded`
- 到达 cooldown -> 半开探测
- 探测成功 -> `active`
- 探测失败 -> 延长 cooldown

#### 8.3 与主链路解耦

健康巡检不应阻塞 runner 主流程；可由后台 goroutine 或显式调度触发，但第一轮只做 manager 内部轻量调度即可。

### 测试建议

- 熔断阈值触发
- 半开恢复
- 单扩展故障下 builtin tools 持续可用

命令可先跑：

- `go test ./internal/extensions -run 'Health|Recovery|Isolation' -v`
- `go test ./internal/extensions -race`

### 完成标志

1. 单扩展故障不会拖垮全局工具体系。
2. 扩展可以自动恢复到 active。

---

## 阶段 9：管理入口与配置模型

### 目标

给使用者提供最小可运维入口，但建议 **先 CLI，后 TUI**。

### 为什么先 CLI

PRD 写了“修改 `internal/tui/*` 增加 ext 子命令”，但当前系统入口更清晰的落点其实不在 `cmd/bytemind/main.go`，而是在 `internal/app/cli_dispatch.go:15` 的命令分发层；现有顶层命令只有 `chat`、`tui`、`run`、`install`、`help`。因此这一阶段必须先明确命令接入方式，再决定是否需要改 TUI。

### 命令入口建议

建议在方案里先固定一种路径，避免阶段 9 才临时决定：

- **推荐**：新增一级 CLI 命令 `bytemind ext ...`，在 `internal/app/cli_dispatch.go:23` 增加 `ext` 分支，再下沉到独立 handler。
- 备选：先做 chat/TUI 内 slash command，把扩展管理放入已有交互流。

优先推荐一级 CLI 命令，因为更符合自动化测试、脚本化运维和错误码校验需求。

### 具体改造

建议优先支持：

- `bytemind ext list`
- `bytemind ext load`
- `bytemind ext unload`
- `bytemind ext status`

配置侧新增：

- `internal/config` 中 extensions 配置结构
- `docs/extensions/config.md`

### 配置建议

按 PRD 的配置草案实现：

- `sources`
- `auto_load`
- `health_check_interval_sec`
- `failure_threshold`
- `recovery_cooldown_sec`
- `max_concurrency_per_extension`
- `conflict_policy`

第一轮只要把配置加载和默认值处理做好，不必一开始就支持热更新 watcher。

### 测试建议

- config 解析
- CLI 子命令输出
- 参数解析与错误码
- auto-load 行为

命令可先跑：

- `go test ./internal/config -v`
- `go test ./internal/app -run Ext -v`

### 完成标志

1. 用户可在 CLI 查询扩展状态。
2. 启动时可按配置自动加载。
3. `ext` 命令的参数解析与错误码有测试覆盖。

---

## 阶段 10：合同测试、E2E 与门禁

### 目标

为 extension 演进建立回归安全网。

### 建议新增

- `internal/extensions/contract_test.go`
- `internal/extensions/e2e_test.go`
- `internal/extensions/testdata/*`
- `internal/tools/registry_contract_test.go`

### 必测项

1. registry contract：结构化错误、默认 reject、冲突可追踪。
2. registry concurrency：并发 register/unregister/get/list + `-race`。
3. lifecycle：非法状态迁移拦截。
4. bridge：builtin vs extension 同名冲突。
5. E2E：`discover -> load -> bridge -> execute -> degrade -> recover`。

### CI 建议

至少增加：

- `go test ./internal/tools ./internal/extensions ./internal/skills ./internal/policy -race`
- 视耗时再补 `go test ./...`

### 完成标志

1. extension 关键合同破坏会直接暴露。
2. race 问题成为强门禁。

---

## 落地前置清单

在进入批次开发前，建议先把下面四项定掉，否则后续设计仍会反复返工：

1. **现状校准**：明确真实存在的包、文件与调用链，尤其是 `internal/tools`、`internal/extensions`、`internal/skills`、`internal/policy`、`internal/app`。
2. **registry 兼容迁移策略**：明确旧 API（`Add`）保留周期、新 API（`Register/Unregister`）接入顺序，以及 builtin fail-fast 与 extension 结构化错误的边界。
3. **stable key 约束与 contract test**：先定义字符集、长度、归一化与 provider 兼容规则，再冻结最终格式。
4. **skills 快照读取接口**：先设计不触发 reload 的只读 API，再做 adapter cache，避免热路径被底层 `Reload()` 抵消。

## 推荐的实际落地批次

为了贴合当前项目，而不是一次做满全部 PRD，推荐拆成 4 个可提交批次：

### 批次 A：底座批

范围：

- registry 契约化
- registry 并发安全
- extensions 域模型
- manager 基础骨架

结果：

- 系统拥有可承载 extension 的基础设施
- 但还未真正接入外部能力

### 批次 B：skills 接入批

范围：

- discovery/validator 第一版
- skills adapter
- bridge 第一版
- policy stable key 打通

结果：

- 现有 skills 被纳入统一 extension 管理
- 形成第一条真实端到端链路

### 批次 C：稳定性批

范围：

- lifecycle 完整化
- isolation/recovery
- 健康检查
- CLI 管理命令

结果：

- extension 从可用变成可运维

### 批次 D：MCP 接入批

范围：

- MCP adapter
- MCP health
- E2E 与 CI 门禁补全

结果：

- 满足 PRD 的 MCP + Skills 统一接入目标

## 不建议当前就做的事项

1. 不建议一开始就引入复杂持久化状态存储；内存态足够支撑当前阶段。
2. 不建议先做 TUI 复杂面板；CLI 管理命令更适合作为第一入口。
3. 不建议在 executor 或 runner 中增加 extension 分支逻辑；这会破坏当前主链路清晰度。
4. 不建议直接大改 `internal/skills/manager.go` 的外部行为；优先通过 adapter 和 cache 包裹。
5. 不建议为了 manifest 校验马上引入重型依赖，除非已有明确 schema 运行时需求。

## 验收口径

当下面这些条件都成立时，可以认为 extension 迭代达到第一阶段可交付状态：

1. registry 不再因扩展注册错误触发 `panic`。
2. extension manager 能返回一致性的扩展快照。
3. 至少一种真实来源（优先 skills）可被发现、加载、列出、卸载。
4. extension tool 能用稳定键注册进 registry。
5. builtin 与 extension 同名冲突可被拒绝并定位。
6. 单扩展失败只影响自身状态，不影响 builtin tools 执行。
7. `internal/tools`、`internal/extensions`、`internal/skills` 相关测试可稳定通过，并完成 `-race` 验证。

## 推荐先改的文件清单

优先级从高到低：

1. `internal/tools/registry.go`
2. `internal/tools/registry_test.go`
3. `internal/extensions/manager.go`
4. `internal/extensions/manager_test.go`
5. `internal/extensions/types.go`（新）
6. `internal/extensions/interface.go`（新）
7. `internal/extensions/errors.go`（新）
8. `internal/extensions/discovery.go`（新）
9. `internal/extensions/validator.go`（新）
10. `internal/extensions/skills/adapter.go`（新）
11. `internal/extensions/bridge.go`（新）
12. `internal/config/*`
13. `cmd/bytemind/*` 或命令分发入口

## 最终建议

如果按当前仓库现实情况推进，最优顺序不是“先全量实现 PRD 所有模块”，而是：

**Registry 改造 -> Extensions 域模型 -> Skills adapter -> Bridge/policy stable key -> Lifecycle/Recovery -> CLI/config -> MCP adapter -> E2E/CI**

这样风险最低、复用最高，也最符合当前代码结构与测试基础。