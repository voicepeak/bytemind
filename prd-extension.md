## 0. 文档信息

- 产品：ByteMind
- 模块：Extension（`internal/extensions`）
- 文档版本：PRD 2.2
- 文档日期：2026-04-15
- 适用代码基线：`main`（当前本地主线）
- 关联架构基线：PR #162（架构设计文档）
- 目标读者：产品、架构、研发、测试、运维

## 1. 背景与目标

ByteMind 已有工具体系与 skills 机制，但扩展接入仍分散。Extension 的目标是成为外部能力接入层：统一把 MCP/Skills 映射为标准 Tool，提供可加载、可停用、可观测、可降级、可恢复的接入能力。

## 2. 架构边界（强约束）

### 2.1 In Scope

1. 扩展发现、加载、卸载。
2. 扩展 manifest/schema/version 校验。
3. MCP/Skills 能力映射到标准 Tool 并注册到 `tools`。
4. 扩展生命周期管理（loaded/active/degraded/stopped）。
5. 扩展故障隔离与恢复。
6. 扩展状态查询与运行事件上报。
7. 扩展到 `tools/policy` 的 source-aware 命名桥接。

### 2.2 Out of Scope

1. 权限决策（allow/deny/ask）与风险规则本身。
2. 审计日志持久化。
3. 主循环触发编排。
4. 任务状态机与重试编排。
5. provider 路由与模型调用治理。

### 2.3 协作约定与硬约束

1. Extension 只产出 Tool 能力；执行前权限判定由 `policy + tools` 完成。
2. Extension 只上报事件；是否落盘由 `agent/storage` 负责。
3. Extension 不直接读写 `agent` 内部状态。
4. Extension 不得导致全局注册器崩溃。
5. 单扩展失败只能影响自身生命周期状态，不得影响其他扩展与 builtin 工具可用性。

## 3. 当前基线与缺口

### 3.1 现有可复用能力

1. `internal/skills/manager.go`（skills 发现与解析）。
2. `internal/tools/registry.go`（工具注册）。
3. `internal/tools/executor.go`（工具执行）。
4. `internal/agent/runner.go`（工具调用主链路）。

### 3.2 缺口

1. MCP 与 Skills 缺少统一契约。
2. `registry` 的错误语义与并发契约不足以支撑 extension 大规模接入。
3. 缺少 extension 生命周期状态机。
4. 缺少 extension 级健康检查与隔离机制。
5. 缺少 extension -> tools -> policy 的稳定命名桥接。
6. 缺少 extension 合同测试、并发 race 门禁与同名兼容回归基线。

## 4. 目标指标（上线后 90 天）

1. MCP/Skills 统一接入覆盖率：100%。
2. 扩展加载成功率：>= 99%。
3. 单扩展故障隔离成功率：>= 98%。
4. 扩展注册耗时 P95：<= 500ms。
5. 故障扩展恢复到 active 的耗时 P95：<= 30s。

## 5. 契约与数据模型

### 5.1 核心类型

```go
type ExtensionKind string
const (
    ExtensionMCP   ExtensionKind = "mcp"
    ExtensionSkill ExtensionKind = "skill"
)

type ExtensionStatus string
const (
    StatusLoaded   ExtensionStatus = "loaded"
    StatusActive   ExtensionStatus = "active"
    StatusDegraded ExtensionStatus = "degraded"
    StatusStopped  ExtensionStatus = "stopped"
)

type Manifest struct {
    ID           string
    Name         string
    Kind         ExtensionKind
    Version      string
    Description  string
    Entry        string
    Capabilities []Capability
    UpdatedAt    time.Time
}
```

### 5.2 核心接口

```go
type Extension interface {
    Info() ExtensionInfo
    Activate(ctx context.Context, opts ActivateOptions) error
    Deactivate(ctx context.Context) error
    Health(ctx context.Context) (ExtensionStatus, error)
}

type Manager interface {
    Load(ctx context.Context, source string) (ExtensionInfo, error)
    Unload(ctx context.Context, extensionID string) error
    List(ctx context.Context) ([]ExtensionInfo, error)
    Get(ctx context.Context, extensionID string) (ExtensionInfo, bool, error)
}

type Resolver interface {
    ResolveTools(ctx context.Context, extensionID string) ([]ExtensionTool, error)
}
```

### 5.3 Registry 契约（前置约束）

1. `internal/tools/registry.go` 的 `Add/Register` 必须从 `panic` 改为返回结构化错误。
2. 同名冲突默认策略必须为 `reject`，禁止 silent overwrite。
3. registry 全路径（register/unregister/get/list）必须并发安全（`RWMutex` 或等价机制）。
4. `List/Get` 必须满足一致性快照语义，不得暴露部分写入态。
5. 推荐最小错误字段：`code/message/tool_key/source/extension_id/cause`。

### 5.4 稳定命名键与策略键

1. Tool 稳定命名键：`stable_tool_key = source + ":" + extensionID + ":" + toolName`。
2. Policy 必须使用 source-aware key，避免 extension 与 builtin 同名污染权限语义。
3. 冲突拒绝错误必须包含可定位上下文：`source/extension_id/tool_name/conflict_with`。

### 5.5 FR-EXT-006 事件契约（最小集合）

最小事件集：`load`、`activate`、`degraded`、`recover`、`unload`。

最小字段集：

1. `extension_id`
2. `kind`
3. `status`
4. `reason`
5. `error_code`
6. `occurred_at`

## 6. 功能需求（FR）

- `FR-EXT-001`：支持 MCP 与 Skills 统一接入。
- `FR-EXT-002`：支持生命周期管理与状态查询。
- `FR-EXT-003`：支持 manifest/schema/version 校验。
- `FR-EXT-004`：支持能力映射为标准 Tool 并注册。
- `FR-EXT-005`：支持扩展健康检查与故障隔离。
- `FR-EXT-006`：输出统一扩展运行事件（事件集与字段集遵循 5.5）。
- `FR-EXT-007`：Registry 注册冲突默认拒绝，且错误可观测、可追踪、可定位。
- `FR-EXT-008`：并发 load/unload/list/get 满足一致性快照语义。

## 7. 实施步骤（执行顺序）

### Step 0：Registry 契约与并发安全改造（前置）

**目标**

先完成 `internal/tools/registry.go` 的基础契约改造，作为 extension 实施前置门槛。

**代码文件**

1. 修改 `internal/tools/registry.go`。
2. 新增/修改 `internal/tools/registry_test.go`。
3. 新增 `internal/tools/registry_contract_test.go`。

**接口实现**

1. `Add/Register` 由 `panic` 改为结构化 `error` 返回。
2. 增加冲突策略，默认 `reject`。
3. 增加并发读写保护与一致性快照读取。

**失败处理**

1. 名称冲突：返回 `duplicate_name`，附带冲突键与来源。
2. 非法来源：返回 `invalid_source`。
3. schema 非法：返回 `invalid_schema`。

**测试点**

1. contract：结构化错误字段完整性。
2. contract：默认 reject，不发生 silent overwrite。
3. concurrency：并发 register/unregister/get/list 无脏读。

**完成定义**

1. 扩展注册失败不触发进程级崩溃。
2. 并发场景无 `concurrent map write`。

---

### Step 1：建立 `internal/extensions` 统一域模型

**目标**

统一 extension 概念与接口，消除 skills/MCP 各自为政。

**代码文件**

1. 新增 `internal/extensions/types.go`。
2. 新增 `internal/extensions/manifest.go`。
3. 新增 `internal/extensions/interface.go`。
4. 新增 `internal/extensions/errors.go`。

**接口实现**

1. 定义 `ExtensionKind/Status/ErrorCode`。
2. 定义 `Manifest/ExtensionInfo/Capability`。
3. 定义 `Extension/Manager/Resolver`。

**测试点**

1. 类型与状态枚举序列化一致性。
2. 接口契约向后兼容测试。

---

### Step 2：实现发现与校验管线

**目标**

扩展源可发现、可校验、可拒绝非法输入。

**代码文件**

1. 新增 `internal/extensions/discovery.go`。
2. 新增 `internal/extensions/validator.go`。
3. 新增 `internal/extensions/schema/manifest.schema.json`。

**接口实现**

1. `Discover(sources []Source) ([]ManifestRef, error)`。
2. `Validate(ref ManifestRef) (Manifest, error)`。

**失败处理**

1. 文件不可读：记录错误并跳过该源。
2. schema 不合法：拒绝加载并给字段级报错。
3. 版本不兼容：标记 `incompatible_version`。

**测试点**

1. 多源扫描顺序与去重。
2. schema 校验错误信息可读性。
3. 不兼容版本阻断行为。

---

### Step 3：实现生命周期管理器

**目标**

支持 loaded/active/degraded/stopped 的安全迁移与并发一致性。

**代码文件**

1. 新增 `internal/extensions/manager.go`。
2. 新增 `internal/extensions/lifecycle.go`。
3. 新增 `internal/extensions/state_store.go`（内存态）。

**接口实现**

1. `Load`：校验 -> 初始化 -> loaded。
2. `Activate`：依赖就绪 -> active。
3. `Deactivate/Unload`：资源释放 -> stopped。
4. `List/Get`：返回一致性快照。

**生命周期约束**

1. 禁止非法跃迁：`stopped -> degraded`。
2. 并发 `load/unload/list/get` 在快照层面保证读一致。

**失败处理**

1. 并发重复加载：返回 `already_loaded`。
2. 卸载中执行：返回 `busy` 并拒绝状态跃迁。

**测试点**

1. 并发 load/unload 竞态。
2. 非法状态迁移拦截。
3. 状态查询一致性。

---

### Step 4：实现 MCP Adapter

**目标**

把 MCP 服务稳定映射为 extension 工具集合。

**代码文件**

1. 新增 `internal/extensions/mcp/adapter.go`。
2. 新增 `internal/extensions/mcp/client.go`。
3. 新增 `internal/extensions/mcp/health.go`。

**接口实现**

1. `FromMCPServer(cfg) (Extension, error)`。
2. `ResolveTools` 从 MCP 能力生成 `ExtensionTool`。
3. `Health` 检查连接、握手、能力可用性。

**失败处理**

1. 握手失败：标记 degraded，不阻塞其他扩展。
2. 工具声明缺字段：仅跳过该工具，不崩溃整个扩展。

---

### Step 5：实现 Skills Adapter（含缓存策略）

**目标**

把 legacy skills 纳入统一 extension 管理，且满足热路径性能约束。

**代码文件**

1. 新增 `internal/extensions/skills/adapter.go`。
2. 新增 `internal/extensions/skills/compat.go`。
3. 新增 `internal/extensions/skills/cache.go`。
4. 复用 `internal/skills/manager.go`。

**接口实现**

1. `FromSkill(skillDef) Extension`。
2. `NormalizeLegacySkill` 做字段补全。
3. 实现缓存 + 增量刷新 + 失效策略。

**性能约束**

1. 禁止在 `List/Find` 热路径做全量 reload。
2. 失效触发至少覆盖：配置变更、源变更、显式 unload。

**测试点**

1. 旧 skill 无改造兼容。
2. 缓存命中路径时延对比。
3. 增量刷新与失效正确性。

---

### Step 6：实现工具桥接（包含 source-aware 命名）

**目标**

将 ExtensionTool 无缝注册到 tools registry，并与 policy 键语义对齐。

**代码文件**

1. 新增 `internal/extensions/bridge.go`。
2. 修改 `internal/tools/registry.go`。
3. 新增 `internal/extensions/bridge_test.go`。

**接口实现**

1. `Bridge(extensionTool) tools.Tool`。
2. 生成稳定键：`source:extensionID:toolName`。
3. policy 查询与透传使用 source-aware key。

**失败处理**

1. schema 不合法：拒绝注册。
2. 名称冲突：默认 reject，并返回可追踪冲突信息。

**测试点**

1. schema 校验路径。
2. builtin vs extension 同名冲突。
3. 事件透传正确性。

---

### Step 7：故障隔离与恢复

**目标**

单扩展故障可隔离，系统整体可持续运行。

**代码文件**

1. 新增 `internal/extensions/isolation.go`。
2. 新增 `internal/extensions/recovery.go`。
3. 新增 `internal/extensions/health_manager.go`。

**接口实现**

1. 失败计数、熔断窗口、半开探测。
2. 恢复重试采用指数退避。
3. 按扩展维护独立健康状态。

**失败处理**

1. 高频失败：进入熔断窗口（degraded）。
2. 半开探测失败：延长隔离并指数退避。

**测试点**

1. 熔断阈值触发。
2. 半开恢复策略。
3. 单扩展故障对全局影响评估（其他工具持续可用）。

---

### Step 8：管理入口与配置模型

**目标**

提供可操作的扩展运维入口。

**代码文件**

1. 修改 `internal/tui/*` 增加 `ext` 子命令。
2. 修改 `internal/config/*` 增加 extensions 配置。
3. 新增 `docs/extensions/config.md`。

**接口实现**

1. `ext list/load/unload/status` 命令。
2. 启动日志输出扩展装配摘要。

**测试点**

1. CLI/TUI 命令回归。
2. 配置热更新与重载。

---

### Step 9：合同测试 / E2E / CI 门禁

**目标**

让扩展模块可持续演进且可回归验证。

**代码文件**

1. 新增 `internal/extensions/testdata/*`。
2. 新增 `internal/extensions/contract_test.go`。
3. 新增 `internal/extensions/e2e_test.go`。
4. 修改 CI 配置加入 extension 测试门禁。

**必测门禁**

1. registry contract（结构化错误、默认 reject、冲突可追踪）。
2. registry concurrency + race（并发 load/unload/list/get + `-race`）。
3. policy key 兼容性回归（builtin vs extension 同名）。
4. E2E：`discovery -> load -> bridge -> execute -> degrade -> recover`。

**失败处理**

1. 合同破坏：CI 直接失败。
2. race 检测失败：CI 直接失败，不允许降级放行。
3. 波动性用例：仅允许重试一次并保留日志。

## 8. 配置模型（示例）

```yaml
extensions:
  sources:
    - type: local
      path: ~/.bytemind/extensions
    - type: workspace
      path: ./.bytemind/extensions
  defaults:
    auto_load: true
    health_check_interval_sec: 30
    failure_threshold: 3
    recovery_cooldown_sec: 20
    max_concurrency_per_extension: 4
    conflict_policy: reject
```

## 9. 测试与验收方案

1. 单元测试：校验器、状态机、桥接器、错误码映射、registry contract。
2. 集成测试：MCP/Skills 到 tools 的端到端链路。
3. 并发测试：高并发 load/unload/list/get 一致性快照与 race 检测。
4. 故障测试：网络抖动、扩展超时、扩展崩溃、自动恢复。
5. 性能测试：100 扩展并存的加载与巡检开销；`List/Find` 热路径缓存命中性能。
6. 兼容回归：builtin 与 extension 同名工具的 policy key 行为一致性。

## 10. 风险与回滚

1. 风险：legacy skills 行为差异。
   - 应对：兼容适配层 + 灰度开关。
2. 风险：MCP 连接不稳定。
   - 应对：扩展级熔断和恢复，不影响其他扩展。
3. 风险：扩展规模上升导致巡检压力。
   - 应对：分片巡检与限流 + 缓存增量刷新。

回滚策略：

1. 保留旧 `skills -> tools` 直连开关。
2. 按来源单独关闭 bridge（mcp/skill）。
3. 出现重大问题时回退到仅内建 tools 运行模式。

## 11. 最终验收标准（DoD）

1. MCP 与 Skills 均可统一加载、管理、桥接到 tools。
2. 生命周期状态完整可见（loaded/active/degraded/stopped）。
3. 单扩展故障可隔离且系统主链路可继续。
4. extension 模块不包含权限决策逻辑。
5. extension 模块不包含审计持久化逻辑。
6. 扩展注册失败不触发进程级崩溃。
7. 并发压测下无 `data race` / `concurrent map write`。
8. 同名工具冲突可观测且可追踪（拒绝原因可定位）。

## 附录 A：与当前代码映射（首批改造点）

1. `internal/tools/registry.go`：先完成 Step 0 契约与并发安全改造。
2. `internal/skills/*`：接入 skills adapter（含缓存与增量刷新）。
3. `internal/tools/executor.go`：保持统一执行，不引入 extension 特判。
4. `internal/agent/runner.go`：仅消费工具能力，不感知扩展内部实现。
5. 新增目录：`internal/extensions/*`。
