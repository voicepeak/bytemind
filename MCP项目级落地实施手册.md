# ByteMind MCP 项目级落地实施手册（无时间版）

## 0. 角色定位与边界

先统一定位，避免方案执行时跑偏：

1. ByteMind 是 **MCP 接入方（Host/Client）**，目标是让用户接入外部 MCP Server。
2. ByteMind 不是 MCP 供应方，不承担第三方服务市场或托管平台职责。
3. 本方案聚焦“接入能力产品化”：连接、治理、可观测、可回滚。

对外能力边界：

1. 支持用户配置并接入自有/第三方 MCP Server。
2. 把 MCP tools 融入 ByteMind 现有工具与审批体系。
3. 不负责替用户采购 MCP 供应商服务。

---

## 1. 文档目标

这份手册用于把 ByteMind 的 MCP 能力从“设计预留”落地为“可运行、可观测、可回滚”的项目能力。

目标要求：

1. 支持 MCP `stdio` 服务接入（首版仅实现 `initialize`、`tools/list`、`tools/call`）。
2. MCP 工具统一注册到现有 `tools.Registry`，复用现有审批、策略、审计、执行链路。
3. MCP 扩展状态可观测（`ready/degraded/failed/stopped`），故障不拖垮主流程。
4. 保持现有 prompt 组装主链不变，仅补充可用工具集合。
5. 支持一键关闭 MCP（feature flag），确保可回退。

非目标（本轮不做）：

1. 不接入 MCP `resources/*` 和 `prompts/*`。
2. 不做 HTTP/SSE transport。
3. 不做插件市场分发。
4. 不改现有 agent 主循环语义。

---

## 2. 现状基线（落地前确认）

先确认当前代码形态，避免方案与现状错位：

1. `internal/extensions/types.go` 已有 `ExtensionMCP` 枚举，但只是类型预留。
2. `internal/extensions/manager.go` 当前实际只通过 `skills` 管线发现和管理扩展。
3. `internal/extensions/skills_adapter.go` 输出 `Kind` 固定是 `ExtensionSkill`。
4. `internal/app/bootstrap.go` 会创建 `extensionspkg.NewManager(workspace)` 并注入 `Runner`。
5. `internal/tools/registry.go`、`internal/tools/executor.go` 已具备可复用的工具注册、权限、执行框架。
6. `internal/extensions/interface.go` 的 `Manager` 目前仅暴露 `Load/Unload/Get/List`，`Load` 参数仍是 `source string` 语义。
7. `internal/extensions/lifecycle.go` 当前迁移以 `loaded/active/degraded/stopped` 为主，`ready/failed` 尚未形成完整迁移约束。

这意味着：MCP 不需要重写主框架，只需新增一个 MCP adapter 并接入 extension manager 与 registry 生命周期。

---

## 2.1 入口策略（必须三层并存）

为了同时覆盖“可用性 + 自动化 + 可维护性”，MCP 接入入口采用三层：

1. **配置层（真实来源）**  
   `config` 是最终状态来源，负责持久化、可审计、可版本化。
2. **交互层（TUI slash）**  
   提供 `/mcp` 命令做会话内操作，降低用户接入门槛。
3. **自动化层（CLI）**  
   提供 `bytemind mcp ...`，用于脚本、CI、批量初始化。

设计原则：

1. slash/CLI 最终都落到配置层。
2. 任何临时态都要可回放为配置变更。
3. 不允许“仅内存生效、重启丢失”的接入行为。

---

## 2.2 统一 MCPService 后端契约（CLI/slash/TUI 共用）

为避免三套入口各自实现分叉，新增统一服务接口（命名可调整，但方法集固定）：

1. `List(ctx)`：列出 MCP server 状态与摘要。
2. `Add(ctx, req)`：新增配置并持久化。
3. `Remove(ctx, serverID)`：删除配置并卸载。
4. `Enable(ctx, serverID)`：启用并触发加载。
5. `Disable(ctx, serverID)`：停用并卸载。
6. `Test(ctx, serverID)`：握手与 `tools/list` 健康测试。
7. `Reload(ctx, serverID|all)`：重载配置并同步工具。

约束：

1. CLI/slash/TUI 只做参数解析与展示，不直接改 manager 内部状态。
2. 配置写回、进程控制、健康检查都由 `MCPService` 承担。
3. `Manager.Load/Unload` 不承载配置写回语义，避免接口污染。

---

## 3. 目标目录与文件改动清单

### 3.1 新增文件

1. `internal/extensions/mcp/types.go`
2. `internal/extensions/mcp/config.go`
3. `internal/extensions/mcp/protocol.go`
4. `internal/extensions/mcp/client_stdio.go`
5. `internal/extensions/mcp/session.go`
6. `internal/extensions/mcp/adapter.go`
7. `internal/extensions/mcp/tool_adapter.go`
8. `internal/extensions/mcp/health.go`
9. `internal/extensions/mcp/errors.go`
10. `internal/extensions/mcp/testdata/stub_server/main.go`
11. `internal/extensions/mcp/*_test.go`

### 3.2 修改文件

1. `internal/config/config.go`
2. `config.example.json`
3. `internal/extensions/manager.go`
4. `internal/extensions/interface.go`（如需扩展接口能力）
5. `internal/app/bootstrap.go`
6. `internal/agent/engine_run_setup.go`
7. `tui` 下 slash 命令与命令面板文件（新增 `/mcp` 命令体系）
8. `cmd/bytemind` 下 CLI 子命令路由（新增 `mcp` 子命令）

---

## 4. 实施步骤（逐步执行）

## Step A：扩展配置模型（Config 层）

### A.1 添加配置结构

在 `internal/config/config.go` 的 `Config` 里新增 `MCP MCPConfig` 字段，并新增以下结构：

- `MCPConfig`
- `MCPServerConfig`
- `MCPTransportConfig`
- `MCPToolOverrideConfig`

建议字段：

1. `enabled`：总开关。
2. `servers[]`：服务列表。
3. `server.id`：唯一标识，后续映射 `extension_id`。
4. `server.transport.type`：本轮固定支持 `stdio`。
5. `server.transport.command/args/env/cwd`：启动命令配置。
6. `startup_timeout_s`、`call_timeout_s`。
7. `max_concurrency`。
8. `auto_start`。
9. `tool_overrides`：按工具覆盖 `safety_class/read_only/destructive/allowed_modes/...`。

### A.2 默认值与归一化

在 `Default()` 与 `normalize()` 里补齐默认值：

1. `mcp.enabled` 默认 `false`。
2. timeout 默认有边界，禁止 0 或负数。
3. `max_concurrency` 最小值保护。
4. `server.id` 必须唯一、非空、规范化（小写、去空格）。
5. `transport.type` 非 `stdio` 时直接报错（本轮范围内）。

### A.3 示例配置

更新 `config.example.json`，加入最小可用 MCP 示例（但默认可以是 `enabled: false`）。

配置模板（建议）：

```json
{
  "mcp": {
    "enabled": false,
    "servers": [
      {
        "id": "github",
        "transport": {
          "type": "stdio",
          "command": "npx",
          "args": ["-y", "@modelcontextprotocol/server-github"],
          "env": {
            "GITHUB_TOKEN": "${GITHUB_TOKEN}"
          }
        },
        "auto_start": true,
        "startup_timeout_s": 20,
        "call_timeout_s": 60,
        "max_concurrency": 4
      }
    ]
  }
}
```

### A.4 单元测试

新增/更新 `internal/config/config_test.go`：

1. 解析成功：完整 MCP 配置。
2. 解析失败：重复 `server.id`。
3. 解析失败：`transport.type` 非法。
4. 默认值补齐正确。
5. `mcp.enabled=false` 时对现有逻辑无影响。

---

## Step B：实现 MCP 协议与 stdio 客户端

### B.1 定义协议消息

在 `internal/extensions/mcp/protocol.go` 定义 JSON-RPC 2.0 请求/响应结构，至少覆盖：

1. `initialize`
2. `tools/list`
3. `tools/call`

要求：

1. 支持 `id` 关联。
2. 标准错误对象解析。
3. 参数与结果保留 `json.RawMessage`。

### B.2 实现 stdio 会话

在 `client_stdio.go/session.go` 实现：

1. 启动子进程（`exec.CommandContext`）。
2. 写入 stdin（每条 JSON 一行）。
3. 读取 stdout 并按行解码 JSON-RPC。
4. 维护 pending map：`request_id -> chan response`。
5. 支持请求超时与上下文取消。
6. 进程退出时统一清理 pending 并返回可诊断错误。

### B.3 初始化握手

连接后执行：

1. `initialize`
2. 校验返回能力至少包含 tool 调用所需字段。
3. 失败则标记 server `degraded/failed`，但不影响主进程继续运行。

### B.4 客户端单测（含 stub server）

通过 `testdata/stub_server` 验证：

1. 正常握手。
2. `tools/list` 正常。
3. `tools/call` 正常。
4. 超时路径。
5. server 崩溃后错误路径。
6. 并发调用正确路由到各自响应。

---

## Step C：把 MCP Tool 映射到 ByteMind Tool

### C.1 定义 Tool 映射模型

在 `tool_adapter.go` 中定义 `MCPToolDescriptor -> toolspkg.Tool` 转换。

映射规则：

1. `ToolDefinition.Function.Name` 使用稳定 key：
   - `mcp:<server_id_normalized>:<tool_name_normalized>`
2. 原始名称保存在 metadata：
   - `OriginalName = <tool_name>`
3. `ExtensionID = mcp.<server_id>`
4. 输入 schema 直接透传（若为空，降级到 object schema）。

### C.2 SafetyClass 策略

默认策略：

1. MCP tool 默认 `SafetyClassSensitive`。
2. 可由 `tool_overrides` 覆盖。
3. 覆盖值必须通过 `ValidateToolSpec`。

### C.3 Tool.Run 实现

`Run(ctx, raw, execCtx)` 内做：

1. 参数 `raw` 透传给 MCP `tools/call`。
2. 应答序列化为字符串返回。
3. 错误统一映射为 `ToolExecError`（优先使用 `permission_denied` / `timeout` / `tool_failed`）。

### C.4 注册到 Registry

调用 `extensions.RegisterBridgedToolWithOptions(...)`：

1. `Source = ExtensionMCP`
2. `ExtensionID = mcp.<server_id>`
3. 允许与 builtin 原名同名但 stable key 唯一。

---

## Step D：接入 Extension Manager 生命周期

### D.1 扩展 manager 内部结构

在 `internal/extensions/manager.go` 扩展 state：

1. 维护 `mcpServers` 运行态。
2. 维护每个 server 已注册 tool keys。
3. 与现有 skills state 合并到统一 `ExtensionInfo` 视图。

### D.2 reload 行为

`reload()` 改为双通道：

1. skills discovery（原有逻辑保持）。
2. mcp discovery（来自 config）。

合并规则：

1. ID 唯一：`skill.*` 与 `mcp.*` 不冲突。
2. 任一 mcp server 异常只影响自身 extension status。
3. 其余 extension 必须保持可用。

### D.3 load/unload 行为

`Load/Unload` 扩展支持 MCP：

1. Load：支持通过配置项或 source 定位启动 MCP server。
2. Unload：停止进程、注销 tools、清理状态。
3. 保留原 skill 行为不变。

### D.4 ExtensionInfo 填充

MCP 的 `ExtensionInfo` 需要完整填：

1. `Kind=ExtensionMCP`
2. `Capabilities.Tools` = tools/list 数量
3. `Health` 包含最后错误与检查时间
4. `Manifest.Source.Ref` 指向配置来源（例如 server id 或配置路径片段）

### D.5 生命周期迁移约束（补齐）

将 MCP extension 迁移约束显式化，避免实现时口径不一致：

合法迁移（最小集合）：

1. `loaded -> ready`（配置与连接检查完成）
2. `ready -> active`（可服务）
3. `active -> degraded`（部分能力失效）
4. `active -> failed`（不可服务）
5. `degraded -> ready`（恢复探测通过）
6. `failed -> ready`（重试恢复通过）
7. `loaded|ready|active|degraded|failed -> stopped`（显式停用/卸载）

非法迁移：

1. `stopped -> active|degraded|failed`（需先重新 load）
2. `failed -> active`（必须先经过 ready）

---

## Step E：在 Runner 回合前同步 MCP 工具

### E.1 启动期初始化

在 `internal/app/bootstrap.go`：

1. 构建 extensions manager 时注入 mcp config。
2. 若 `auto_start=true`，在启动后触发一次扩展同步。

### E.2 回合前兜底同步

在 `internal/agent/engine_run_setup.go` 的 `prepareRunPrompt` 前置流程中：

1. 增加“扩展工具已同步”检查。
2. 防止 `tools/list` 已变化但 registry 未更新。

### E.3 prompt 层

`internal/agent/prompt.go` 不改变组装顺序，只通过现有 `AvailableTools` 输出 MCP 稳定工具名。

### E.4 同步节流与降级策略（必须）

避免“每回合 tools/list 全量阻塞拉取”，增加以下规则：

1. 回合前同步加 TTL 缓存（默认 30s，可配置）。
2. 配置变更、`/mcp reload`、`bytemind mcp reload`、连接状态变化触发即时刷新。
3. 刷新失败时回退到最近成功快照，并打标 `reason_code=stale_snapshot_fallback`。
4. 同一 server 在刷新窗口内做 debounce，避免并发重复刷新。

---

## Step F：策略、审批、审计对齐

### F.1 策略复用

不新造策略系统，直接复用现有：

1. `internal/agent/policy_gateway.go`
2. `internal/tools/executor.go`

执行要求：

1. `policyGateway` 是 `Sensitive/Destructive` 风险语义的主判定入口。
2. `executor` 仅执行最终 allow/deny 结果与 destructive 执行审批动作。
3. 非 agent 入口也必须先过统一策略网关，禁止“直连 executor”绕过风险语义。
4. allowlist/denylist 必须作用于 stable tool key。
5. `never` 模式下高风险 MCP 工具应被拒绝并携带明确 `reason_code`。

### F.2 审计补全

调用 MCP tool 时保证审计包含：

1. `tool_name`（stable key）
2. `extension_id`
3. `mcp_server_id`
4. `reason_code`（deny/timeout/error）

兼容约束：

1. 新增字段不得破坏旧查询（缺省值用空串或 `unknown`）。
2. 保持现有 `tool_name/error_code` 查询语义不变。
3. 审计 schema 版本化，支持跨版本回放。

---

## Step G：TUI/CLI 可观测性落地

### G.1 slash 命令层（`/mcp`）

新增 `/mcp` 顶层命令，建议子命令如下：

1. `/mcp list`：列出已配置 server 与状态。
2. `/mcp add <id> --cmd <command> [--args ...]`：新增 server 配置。
3. `/mcp remove <id>`：删除 server 配置。
4. `/mcp enable <id>` / `/mcp disable <id>`：按 server 启停。
5. `/mcp test <id>`：执行握手与 `tools/list` 健康检查。
6. `/mcp reload`：重载配置并同步 tools。
7. `/mcp auth <id>`：引导凭证配置（仅引导，不明文回显 secret）。

命令交互约束：

1. 所有 add/remove/enable/disable 都写回配置文件。
2. 执行后立即触发 manager reload 并返回状态摘要。
3. 错误信息必须可执行（告诉用户下一步修复动作）。

### G.2 命令面板与帮助系统

同步更新：

1. `/help` 中补 MCP 命令用法。
2. 命令面板 `commandItems` 中补 `/mcp` 常用子命令。
3. 保持与现有 `/skills` 风格一致，降低学习成本。

### G.3 CLI 子命令（自动化入口）

在 `cmd/bytemind` 增加 `mcp` 子命令族：

1. `bytemind mcp list`
2. `bytemind mcp add ...`
3. `bytemind mcp remove ...`
4. `bytemind mcp enable/disable ...`
5. `bytemind mcp test ...`
6. `bytemind mcp reload`

CLI 与 slash 要求同一后端服务，不允许两套实现分叉。

### G.4 错误可读性

将 MCP 常见故障归一为可读文案：

1. 启动失败（command 不存在、权限问题）
2. 握手失败（协议不兼容）
3. 调用超时
4. 服务异常退出

### G.5 扩展状态视图

保留 `/extensions`（如已有）作为总览页，显示：

1. `id/kind/status`
2. `tools_count`
3. `last_error`
4. `updated_at`

### G.6 配置写回并发与原子性

对 `/mcp add/remove/enable/disable` 与 CLI 同步约束如下：

1. 写配置采用“临时文件写入 + fsync + rename”原子策略。
2. 进程内写配置全局互斥，防止 TUI 与 CLI 并发覆盖。
3. 检测冲突版本时重试有限次数，失败返回可执行错误提示。
4. 禁止部分写入成功、内存态成功但磁盘失败的不一致结果。

---

## Step H：测试矩阵与验收门禁

## H.1 单元测试

至少覆盖：

1. config 解析与默认值。
2. stdio 客户端请求/响应与超时。
3. tool adapter schema 与 spec 映射。
4. manager 的 load/unload/reload 合并行为。
5. registry 冲突处理与稳定 key 唯一性。

## H.2 集成测试

基于 stub server：

1. 从启动到 tools 注册成功。
2. agent 真正触发 MCP tool call。
3. policy deny/ask/allow 分支。
4. server 故障后 degraded 恢复路径。

## H.3 回归测试（最小必要）

按仓库规则先跑窄测试：

1. `go test ./internal/extensions -v`
2. `go test ./internal/agent -v`
3. `go test ./internal/app -v`
4. 如有变更再扩到全量。

## H.4 DoD（完成定义）

全部满足才算落地完成：

1. `mcp.enabled=false` 时行为与当前版本一致。
2. `mcp.enabled=true` 且 server 可用时，工具可调用并可审计。
3. 单个 MCP server 故障不影响其他扩展和主会话。
4. 能通过配置快速关闭 MCP 并恢复到 skill-only。
5. `/mcp` 与 `bytemind mcp` 操作结果一致，且都可持久化。

---

## 5. 回滚方案（必须可执行）

回滚触发条件：

1. 主链路不可用。
2. 审批策略失效。
3. 大面积 tool 超时/阻塞。

回滚操作：

1. 配置层设置 `mcp.enabled=false`。
2. 重启进程，extensions manager 不再启动 MCP。
3. registry 中仅保留 builtin + skill bridge 工具。
4. 保留日志用于复盘，不删除 session 数据。

---

## 6. 风险与硬约束

1. 子进程管理：任何退出路径都必须回收进程与 goroutine。
2. 并发上限：必须限制每 server 并发，避免 stdout/stderr 堵塞。
3. 协议健壮性：解析失败不可 panic，必须可降级。
4. 工具命名：只能使用 stable key 对外暴露，避免冲突。
5. 安全优先：MCP 不得绕过现有审批与策略判定。

---

## 7. 开发执行清单（按条打勾）

- [ ] 配置结构、normalize、示例配置完成。
- [ ] MCP stdio client 与协议消息完成。
- [ ] stub server 与客户端测试完成。
- [ ] MCP tool adapter 与 spec/safety 映射完成。
- [ ] manager 生命周期集成完成（load/list/unload/reload）。
- [ ] runner 回合同步完成。
- [ ] policy/approval/audit 联通完成。
- [ ] `/mcp` slash 命令完成并接入帮助与命令面板。
- [ ] `bytemind mcp` CLI 子命令完成。
- [ ] TUI 扩展状态展示完成。
- [ ] 回归测试通过并记录结果。
- [ ] 回滚开关验证通过。

---

## 8. 建议的首个可合并提交拆分

建议按可审查粒度拆分：

1. 提交一：`config + schema + tests`
2. 提交二：`mcp stdio client + protocol + stub tests`
3. 提交三：`tool adapter + registry bridge + tests`
4. 提交四：`extension manager 集成 + health`
5. 提交五：`agent/bootstrap/tui/cli 接入 + 文档`

这样每个提交都可独立验证，不会形成超大变更集。

---

## 9. 分期里程碑（风险收敛）

**Phase 1（先闭环）**

1. Config + stdio client + tool bridge。
2. Manager 生命周期接入与基础健康检查。
3. 策略/审批/审计主链打通。
4. 核心回归（extensions/tools/agent）与 race 门禁。

**Phase 2（再扩面）**

1. CLI/slash/TUI 共后端管理能力完善。
2. 扩展状态可观测面与错误文案优化。
3. 完整 E2E 与故障恢复回归。
