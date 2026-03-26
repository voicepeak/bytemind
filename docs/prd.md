# ByteMind MVP 产品需求文档（PRD）

## 1. 产品概述

- **产品名称**：ByteMind
- **一句话定位**：用 Go 构建的零依赖 AI Coding CLI 工具，原生支持国产大模型
- **目标用户**：中国开发者（个人 + 小团队），终端重度用户，Go/Python/TypeScript/Java 等主流语言开发者
- **核心价值主张**：单二进制分发、国产 LLM 一等公民、中文友好、安全可控

### 1.1 产品目标

ByteMind MVP 的目标是在终端内提供一个可独立完成日常编码任务的 Agent CLI。用户应能在单个工具内完成以下闭环：理解代码、搜索上下文、生成或修改代码、执行构建/测试命令、查看 diff、确认写入、记录会话。

### 1.2 MVP 范围边界

**MVP 包含**

1. 交互式 TUI 会话与非交互式单轮执行。
2. 基础 Agent Loop：用户输入、模型响应、工具调用、结果回流、多轮上下文。
3. OpenAI-compatible API 与 Anthropic 原生 API 接入。
4. 内置文件和 Shell 工具集。
5. JSON 日志输出、最大轮次限制。
6. 

**MVP 不包含**

1. VS Code、JetBrains、Neovim 等 IDE 插件。
2. 企业管理后台、组织级权限中心、审计控制台。
3. 插件市场、技能市场、工作流市场。
4. 浏览器自动化、网页抓取、远程桌面类代理能力。
5. 多代理并行协作、子代理调度、任务分叉线程。
6. MCP Server 能力与外部可安装插件协议。
7. 云端托管任务、PR Review Bot、Issue Bot。
8. 团队共享会话、云端同步、多人协作。

### 1.3 产品定位说明

ByteMind 不追求在 MVP 阶段对齐海外竞品的全部能力，而是优先解决中国开发者最真实的终端工作流需求：安装轻、模型接入灵活、中文交互顺滑、文件改动可审阅、命令执行可确认、在公司网络与国产模型环境下可稳定运行。

## 2. 用户场景（User Stories）

### 2.1 场景一：代码生成

**Who**

- 典型用户画像：一名熟悉 Go 和 TypeScript 的后端工程师，习惯在终端中创建新模块、补充接口、生成测试代码，不希望频繁切换到 IDE 插件。

**What**

- 用户希望用自然语言描述需求，让 ByteMind 在项目目录下创建或编辑代码文件，并在写入前展示 diff。

**Why**

- 用户希望缩短样板代码编写时间，同时保留对改动范围和文件落盘的控制权。

**具体操作流程**

1. 用户在项目根目录执行 `bytemind` 进入 TUI。
2. 用户输入“新增一个 Gin 路由 `/healthz`，返回服务版本和时间戳，并补充对应单元测试”。
3. ByteMind 读取 `BYTEMIND.md`、目录结构、相关文件，并提出将修改的文件列表。
4. Agent Loop 调用 `file_read`、`glob`、`grep` 等工具收集上下文。
5. 模型生成修改方案，并通过 `file_write` 或 `file_edit` 形成 patch。
6. TUI 展示逐文件 diff，用户确认后写入。
7. 用户继续要求“运行测试”，工具再发起命令执行确认。

**Acceptance Criteria**

1. 在存在相关项目上下文时，ByteMind 能生成至少一个新文件或修改已有文件。
2. 所有文件写入前都展示 unified diff 预览。
3. 用户拒绝确认时，磁盘上不得产生对应改动。
4. 写入后会话中能看到哪些文件被创建、修改。

**边界情况处理**

1. 如果目标文件不存在且目录不存在，先提示将创建目录与文件，再进入确认。
2. 如果模型生成的补丁无法应用，保留原文件并提示冲突行。
3. 如果用户需求涉及超出当前工作目录的路径，直接拒绝并说明原因。
4. 如果项目已有脏文件且与目标文件重叠，进入脏文件保护流程，不直接覆盖。

### 2.2 场景二：代码理解

**Who**

- 典型用户画像：新加入项目的小团队开发者，需要快速理解模块边界、调用链、配置来源和关键入口。

**What**

- 用户希望围绕现有代码提问，让工具解释实现逻辑、定位符号、搜索相关文件和错误来源。

**Why**

- 用户需要在不手动遍历大量文件的情况下，快速建立项目心智模型。

**具体操作流程**

1. 用户执行 `bytemind`。
2. 输入“解释 `internal/agent` 如何驱动工具调用，入口函数在哪里”。
3. ByteMind 加载当前会话上下文，读取 `BYTEMIND.md`。
4. Agent Loop 调用 `glob`、`list_directory`、`grep`、`file_read`。
5. 模型基于检索结果输出模块解释、调用链和关键文件列表。
6. 用户继续追问“如果要增加一个新工具，最少改哪几个文件”。
7. 会话保留前文上下文，返回后续分析。

**Acceptance Criteria**

1. 用户在一次会话内连续追问时，上下文能被正确继承。
2. 回答中必须引用实际存在的文件和函数，而不是泛化描述。
3. 如果上下文不足，工具必须先搜索或读取文件，再回答。
4. 整个流程中不触发文件写入和 Shell 执行。

**边界情况处理**

1. 如果仓库过大，优先返回最相关的前 N 个文件并说明已截断。
2. 如果 `grep` 无结果，明确告知未命中而不是猜测。
3. 如果用户提问的路径不存在，直接指出无此路径。
4. 如果模型窗口接近上限，提示用户 `/compact` 或自动压缩。

### 2.3 场景三：Bug 调试

**Who**

- 典型用户画像：负责线上故障修复的开发者，手头有一段报错栈、失败测试或复现步骤，希望工具帮忙定位并修复问题。

**What**

- 用户希望输入错误日志或问题描述，让 ByteMind 找到根因、修改代码并执行验证命令。

**Why**

- 用户需要减少人工排查耗时，并让修复过程保留可验证的证据链。

**具体操作流程**

1. 用户进入 TUI 或运行 `bytemind -p "..."`。
2. 粘贴错误信息，例如 panic 栈、HTTP 500 报错或失败测试输出。
3. ByteMind 解析错误关键词，调用 `grep`、`file_read`、`list_directory` 查找相关代码。
4. 模型给出可能原因和修复方案。
5. 工具通过 `file_edit` 生成补丁，展示 diff。
6. 用户确认写入。
7. ByteMind 请求执行 `go test ./...` 或用户指定命令。
8. 执行结果回流模型；若测试仍失败，继续下一轮修复。

**Acceptance Criteria**

1. 用户可直接粘贴错误信息触发检索和定位流程。
2. 修复必须基于项目内实际文件，而不是仅给建议不落地。
3. 测试或构建命令执行结果要显示退出码和摘要。
4. 若修复后测试通过，用户可执行 `/undo` 回滚最近一次 AI 改动。

**边界情况处理**

1. 若错误无法复现且无足够上下文，ByteMind 需明确提出还需要哪些日志或命令输出。
2. 若命令执行超时，允许用户中断并保留已有代码改动。
3. 若修改涉及多个文件，diff 需逐文件展示。
4. 若工作目录不是 Git 仓库，修复仍可进行，但 `/undo` 仅支持本轮文件级回滚，不做 commit 回滚。

### 2.4 场景四：Shell 任务代理

**Who**

- 典型用户画像：习惯在终端里运行构建、测试、格式化、部署前检查命令的工程师或小团队 Tech Lead。

**What**

- 用户希望通过自然语言让 ByteMind 执行终端命令，并把输出摘要回流到会话。

**Why**

- 用户希望减少手动输入命令、复制输出、再粘回 AI 对话的来回切换。

**具体操作流程**

1. 用户在会话中输入“帮我跑一下单元测试，如果失败总结最可能的两个原因”。
2. ByteMind 生成拟执行命令，例如 `go test ./...`。
3. TUI 展示命令、工作目录、超时时间、权限提示。
4. 用户确认后，`bash/shell` 工具开始执行并流式输出。
5. 命令完成后，ByteMind 汇总 stdout/stderr 和退出码。
6. 用户继续要求“只修复编译错误，不处理 lint”。

**Acceptance Criteria**

1. 未经确认时，命令不得执行。
2. 命令执行过程中 stdout/stderr 实时显示。
3. 执行完成后能看到退出码、耗时、摘要。
4. Ctrl+C 可安全中断命令，不退出整个会话。

**边界情况处理**

1. 对 `rm -rf`、`git reset --hard`、生产部署等高风险命令默认拒绝，除非用户显式开启 `auto-accept` 且命令在允许范围内。
2. 如果命令超时，标记为 `timeout`，允许重试或改用更短命令。
3. 如果命令依赖外部工具不存在，返回“命令不可用”而不是继续猜测。
4. 如果命令试图访问工作目录外路径，触发工作目录保护。

### 2.5 场景五：非交互自动化

**Who**

- 典型用户画像：需要在脚本、CI/CD、Git hooks 或批处理任务中调用 AI CLI 的 DevOps 工程师或后端开发者。

**What**

- 用户希望通过 `-p`、管道输入和 JSON 输出，让 ByteMind 在非交互环境中执行单轮或受限多轮任务。

**Why**

- 用户需要把 AI 能力接入流水线，但不能依赖全屏 TUI 或人工持续交互。

**具体操作流程**

1. 用户在脚本中执行 `cat panic.log | bytemind -p "总结根因并给出修复建议" -f json --max-turns 3`。
2. ByteMind 从 stdin 读取附加上下文，把命令行 prompt 作为主任务。
3. 若任务只读，则直接输出结果。
4. 若任务需要写文件或执行命令，则依据 CLI flag 决定拒绝、等待审批或按 `auto-accept` 处理。
5. 输出以 `text`、`json` 或 `stream-json` 格式写到 stdout，stderr 只输出系统错误。

**Acceptance Criteria**

1. `-p` 模式执行完毕后自动退出，退出码可用于脚本判断成功失败。
2. 管道输入与 `-p` 可同时使用，stdin 内容能进入上下文。
3. `-f json` 输出稳定 JSON 结构，`-f stream-json` 逐事件输出。
4. `--max-turns N` 生效，超过轮次后强制停止并返回受限结果。

**边界情况处理**

1. 在无 TTY 场景下若需要人工确认，应直接返回明确错误码和提示。
2. JSON 输出不得混入 Markdown 或彩色控制字符。
3. 当 stdin 为空时，仍允许仅使用 `-p` 执行。
4. 当模型多轮未收敛时，达到 `max-turns` 后返回“未完成”状态和最后一轮摘要。

## 3. 功能需求（Functional Requirements）

### 3.1 Agent Loop 核心

| 功能 ID | 功能名 | 描述 | 优先级 | 验收标准 |
| --- | --- | --- | --- | --- |
| FR-001 | 基础 Agent Loop | 实现输入 → LLM → 工具调用 → 工具结果回流 → 再次推理的循环。 | P0 | 给定一个需要读取文件再回答的问题，系统至少完成 1 次工具调用并产出最终响应。 |
| FR-002 | 流式输出 | LLM 响应必须逐 token 或逐 chunk 实时显示到 TUI/CLI。 | P0 | 在流式模型下，首个输出片段出现于 HTTP 响应建立后，无需等待完整结果。 |
| FR-003 | 工具调用解析与执行 | 支持 OpenAI function calling 与 Anthropic tool_use 两类工具调用协议，统一解析为内部 `ToolCall`。 | P0 | 同一工具定义可被两类 provider 调用并正确执行，错误参数可返回结构化报错。 |
| FR-004 | 多轮对话 | 在一个会话内维护消息历史、工具结果、当前模型与工作目录。 | P0 | 用户连续追问三轮时，系统可引用前两轮上下文，不要求用户重复输入。 |
| FR-005 | 会话中断与恢复 | `Ctrl+C` 仅中断当前生成或工具执行，不退出会话；用户可继续提问。 | P0 | 在流式生成中按 `Ctrl+C` 后，TUI 保持可输入状态，历史消息不丢失。 |

### 3.2 LLM Provider 集成

| 功能 ID | 功能名 | 描述 | 优先级 | 验收标准 |
| --- | --- | --- | --- | --- |
| FR-010 | OpenAI 兼容 API 支持 | 通过 `base_url`、`model`、`api_key` 接入 OpenAI-compatible 接口，覆盖 DeepSeek、通义千问、Groq、OpenRouter、Ollama 等。 | P0 | 用户仅修改配置即可切换到任一兼容 OpenAI Chat Completions/Responses 的服务。 |
| FR-011 | Anthropic Claude 原生支持 | 支持 Claude Messages API、`tool_use`、`extended thinking` 等 Anthropic 特性。 | P1 | 使用 Claude 模型时能成功解析工具调用，并可配置是否启用思考预算。 |
| FR-012 | 模型切换 | 支持启动参数和会话内命令切换模型，不丢失当前会话上下文。 | P0 | 用户执行 `/model deepseek-chat` 后，后续轮次使用新模型，历史消息仍保留。 |
| FR-013 | API Key 管理 | 支持环境变量和配置文件定义多 provider 多 key，并按 provider 路由选择。 | P0 | 配置两个 provider 时，系统能根据当前模型自动选择对应 key；缺失时给出明确报错。 |

### 3.3 内置工具集

| 功能 ID | 功能名 | 描述 | 优先级 | 验收标准 |
| --- | --- | --- | --- | --- |
| FR-020 | bash/shell 执行 | 运行终端命令，流式输出 stdout/stderr，支持超时、中断、退出码收集。 | P0 | 执行 `go test ./...` 时，输出实时刷新；超时后状态为 `timeout`。 |
| FR-021 | file_read | 读取整个文件或指定行范围，返回路径、行号区间和内容。 | P0 | 可读取单文件全部内容或 `start/end` 指定片段，超出行数时安全截断。 |
| FR-022 | file_write | 创建新文件或覆盖写入，用于生成新文件或整文件重写。 | P0 | 写入前必须走 diff 预览确认；拒绝后文件不落盘。 |
| FR-023 | file_edit | 基于 search/replace 精确编辑已有文件，避免模型整文件重写。 | P0 | 指定 search 模式命中唯一位置时成功替换；多处命中时返回歧义错误。 |
| FR-024 | glob | 按 glob 模式列出文件，默认尊重 `.gitignore` 和内置忽略规则。 | P0 | 查询 `**/*.go` 时不返回 `vendor/`、`node_modules/` 等被忽略目录。 |
| FR-025 | grep | 正则搜索文件内容，优先使用 ripgrep 语义，返回文件、行号、匹配片段。 | P0 | 搜索符号或错误字符串时返回最多 N 条匹配，包含路径与行号；`rg` 不可用时有可预期降级。 |
| FR-026 | list_directory | 列出目录内容，支持递归深度限制与文件/目录区分。 | P0 | 用户查询目录结构时，系统能显示排序后的子项，并标记目录与文件。 |

### 3.4 TUI 界面

| 功能 ID | 功能名 | 描述 | 优先级 | 验收标准 |
| --- | --- | --- | --- | --- |
| FR-030 | 全屏 TUI 模式 | 基于 Bubble Tea 实现全屏 TUI，包含消息列表、输入区、状态栏。 | P0 | 运行 `bytemind` 时进入全屏界面，可持续输入多轮消息。 |
| FR-031 | Markdown 渲染 | 支持代码块高亮、表格、列表、引用和内联代码展示。 | P1 | 模型输出 Markdown 时，在 TUI 中可读且代码块具备语法高亮。 |
| FR-032 | 流式消息渲染 | 模型流式输出内容逐步追加到当前消息气泡。 | P0 | 用户能看到回复边生成边追加，而不是最后一次性出现。 |
| FR-033 | 多行输入 | 输入框支持换行编辑，`Ctrl+S` 或 `Enter` 发送，`Shift+Enter` 换行。 | P0 | 用户可粘贴多行需求、错误日志和代码片段，不丢格式。 |
| FR-034 | 工具调用状态展示 | 在界面中显示当前正在执行的工具、阶段、耗时或 spinner。 | P0 | 工具运行时用户能识别是 `grep`、`file_read` 还是 `shell` 正在执行。 |
| FR-035 | Token 用量显示 | 每轮显示 input/output token 和累计近似成本。 | P1 | 一次回复结束后，状态栏或消息尾部能看到 token 用量。 |

### 3.5 权限与安全

| 功能 ID | 功能名 | 描述 | 优先级 | 验收标准 |
| --- | --- | --- | --- | --- |
| FR-040 | 文件编辑确认 | 所有文件写入必须展示 diff 预览，并在用户确认后落盘。 | P0 | 用户选择拒绝时，磁盘不发生变化，会话内记录为 rejected。 |
| FR-041 | 命令执行确认 | 所有 Shell 命令执行前必须展示命令文本、目录和风险提示。 | P0 | 未确认时命令不执行；确认后才启动子进程。 |
| FR-042 | 审批模式 | 支持 `ask` 默认审批与 `auto-accept` 自动接受所有允许操作，用于可信环境。 | P1 | 启动时开启 `--auto-accept` 后，低风险文件写入和命令可直接执行。 |
| FR-043 | 工作目录保护 | 工具操作默认限制在当前项目目录内，防止越界读写。 | P0 | 当工具请求访问父目录或绝对路径越界时，系统拒绝并给出错误原因。 |

### 3.6 Git 集成

| 功能 ID | 功能名 | 描述 | 优先级 | 验收标准 |
| --- | --- | --- | --- | --- |
| FR-050 | 自动 commit | 每次 AI 文件编辑确认后，可按配置自动创建一个 Git commit。 | P1 | 开启自动提交后，写入确认完成即生成 commit，message 带任务摘要。 |
| FR-051 | /undo 回滚 | 提供 `/undo` 回滚最近一次 AI 变更的 Git commit；无 Git 时回退为会话级文件回滚。 | P0 | 最近一次 AI 改动可被单命令撤销，工作树回到前一状态。 |
| FR-052 | Co-authored-by 归因 | 自动 commit message 追加 AI 归因信息。 | P1 | 生成的 commit message 包含 `Co-authored-by: ByteMind` 或配置化署名。 |
| FR-053 | 脏文件保护 | 编辑前若检测到未提交更改，需提示用户 stash、跳过或只修改无冲突文件。 | P0 | 当目标文件已有未提交改动时，系统不会静默覆盖。 |

### 3.7 配置系统

| 功能 ID | 功能名 | 描述 | 优先级 | 验收标准 |
| --- | --- | --- | --- | --- |
| FR-060 | 用户级配置 | 支持 `~/.config/bytemind/config.yaml` 作为默认配置文件。 | P0 | 启动时若存在该文件，系统自动加载并校验字段。 |
| FR-061 | 项目指令文件 | 根目录 `BYTEMIND.md` 自动加载，并作为系统提示的一部分参与每次会话。 | P0 | 仓库根目录放置 `BYTEMIND.md` 后，新会话必定注入该内容。 |
| FR-062 | 环境变量支持 | 支持 `BYTEMIND_API_KEY`、`BYTEMIND_MODEL` 等环境变量覆盖配置。 | P0 | 环境变量存在时优先级高于用户配置。 |
| FR-063 | 模型配置 | 配置 provider、model、base_url、max_tokens、temperature、timeout 等参数。 | P0 | 用户可在 YAML 中设置多 provider 配置并通过模型名选择。 |

### 3.8 非交互模式

| 功能 ID | 功能名 | 描述 | 优先级 | 验收标准 |
| --- | --- | --- | --- | --- |
| FR-070 | `-p "query"` 单轮执行模式 | CLI 接收单轮 prompt，执行后退出，不进入 TUI。 | P0 | `bytemind -p "explain main.go"` 能直接输出结果并返回退出码 0。 |
| FR-071 | 管道输入 | 支持从 stdin 读取附加上下文，与 `-p` 搭配使用。 | P0 | `cat file | bytemind -p "explain this"` 时，stdin 内容进入上下文。 |
| FR-072 | 输出格式选择 | 支持 `-f text`、`-f json`、`-f stream-json`。 | P0 | `-f json` 输出合法 JSON；`-f stream-json` 输出可逐行解析事件流。 |
| FR-073 | `--max-turns N` | 限制 Agent Loop 最大轮次，防止自动模式无限循环。 | P0 | 达到最大轮次后，系统结束执行并返回 `max_turns_reached` 状态。 |

### 3.9 会话命令

| 功能 ID | 功能名 | 描述 | 优先级 | 验收标准 |
| --- | --- | --- | --- | --- |
| FR-080 | `/help` | 显示可用命令列表、说明和示例。 | P0 | 输入 `/help` 后立即返回命令帮助，不触发模型调用。 |
| FR-081 | `/clear` | 清空当前会话上下文，但保留工作目录和配置。 | P0 | 执行后历史消息不再参与后续推理。 |
| FR-082 | `/model <name>` | 切换当前模型。 | P0 | 指定合法模型名后，后续对话使用新模型。 |
| FR-083 | `/undo` | 回滚最近一次 AI 变更。 | P0 | 成功后界面提示回滚的文件或 commit id。 |
| FR-084 | `/quit` / `/exit` | 退出程序，退出前保存会话。 | P0 | 退出后重启程序仍可恢复刚才的会话。 |
| FR-085 | `/compact` | 手动触发上下文压缩，把长会话总结为摘要。 | P1 | 执行后消息历史减少但保留关键信息，后续可继续对话。 |

## 4. 非功能需求（Non-Functional Requirements）

### 4.1 性能

| NFR ID | 指标 | 要求 | 验收方式 |
| --- | --- | --- | --- |
| NFR-001 | 冷启动时间 | `< 200ms` | 在标准开发机上以空项目启动 `bytemind`，记录到 TUI 可输入状态的耗时。 |
| NFR-002 | 首 token 延迟 | `LLM API 延迟 + < 50ms` 框架开销 | 以流式模型调用测量 HTTP 首字节到 TUI 首字符的附加延迟。 |
| NFR-003 | 空闲内存占用 | `< 50MB` | 启动后无活跃任务状态下测量 RSS。 |
| NFR-004 | 单二进制大小 | `< 30MB` | Release 构建产物按目标平台分别统计。 |

### 4.2 兼容性

| NFR ID | 指标 | 要求 | 验收方式 |
| --- | --- | --- | --- |
| NFR-010 | 操作系统支持 | macOS `(arm64/amd64)`、Linux `(amd64/arm64)`、Windows `(amd64)` | CI 构建各平台二进制并运行基础 smoke test。 |
| NFR-011 | 终端兼容 | 支持 256 色及以上现代终端 | 在 iTerm2、Alacritty、Windows Terminal、常见 Linux 终端做手动兼容验证。 |
| NFR-012 | 最小终端尺寸 | 宽 80 列，高 24 行 | 在最小尺寸下 TUI 不应溢出或不可用。 |

### 4.3 安装分发

| NFR ID | 指标 | 要求 | 验收方式 |
| --- | --- | --- | --- |
| NFR-020 | 预编译二进制 | 通过 GitHub Releases 提供各平台包 | 每次 release 自动生成并上传构建产物。 |
| NFR-021 | `go install` | 支持 `go install github.com/xxx/bytemind@latest` | 在全新 Go 环境执行安装并可启动。 |
| NFR-022 | Homebrew | 支持 Homebrew 安装 | 通过 tap 安装后 `bytemind --version` 可执行。 |
| NFR-023 | 一键安装脚本 | 支持 `curl | bash` | 脚本可自动检测平台、下载二进制、校验版本。 |

### 4.4 安全

| NFR ID | 指标 | 要求 | 验收方式 |
| --- | --- | --- | --- |
| NFR-030 | API Key 保护 | API Key 不记录在日志或会话历史中 | 搜索日志和 session 存储中不出现明文 key。 |
| NFR-031 | 文件边界 | 文件操作默认限制在 CWD 及其子目录 | 构造越界路径读写请求，系统必须拒绝。 |
| NFR-032 | 命令权限检查 | Shell 命令执行前必须经过权限检查，除非 `auto-accept` | 在默认配置下，任何命令执行前均弹出确认。 |

### 4.5 可靠性

| NFR ID | 指标 | 要求 | 验收方式 |
| --- | --- | --- | --- |
| NFR-040 | API 重试 | LLM API 调用失败自动重试，最多 3 次，指数退避 | 通过模拟 5xx 或网络抖动验证重试策略。 |
| NFR-041 | 文件持久性 | 异常退出时不丢失已确认的文件变更 | 在写入确认后强制终止进程，文件应保持已写状态。 |
| NFR-042 | 可中断性 | `Ctrl+C` 可随时安全中断，不损坏文件状态 | 在生成中、命令执行中分别中断，文件系统与会话状态保持一致。 |

## 5. CLI 接口设计

### 5.1 命令总览

```bash
bytemind                          # 启动交互式 TUI 会话
bytemind -p "query"               # 非交互模式，单轮执行
bytemind -p "query" -f json       # 非交互模式，JSON 输出
bytemind -m <model>               # 指定模型启动
bytemind -c <dir>                 # 指定工作目录
bytemind --version                # 版本信息
bytemind --help                   # 帮助信息
bytemind config                   # 打开/管理配置
bytemind auth                     # 管理 API Key 认证
```

### 5.2 根命令与全局 flags

| Flag | 缩写 | 类型 | 默认值 | 功能说明 | 使用示例 |
| --- | --- | --- | --- | --- | --- |
| `--prompt` | `-p` | `string` | 空 | 非交互模式主查询；提供后不进入全屏 TUI。 | `bytemind -p "解释 main.go"` |
| `--model` | `-m` | `string` | 配置文件中的默认模型 | 启动时指定模型，覆盖当前会话默认值。 | `bytemind -m deepseek-chat` |
| `--cwd` | `-c` | `string` | 当前 shell 工作目录 | 指定工作目录；所有工具权限边界以此为根。 | `bytemind -c ./services/api` |
| `--format` | `-f` | `enum(text,json,stream-json)` | `text` | 设置非交互输出格式。 | `bytemind -p "summarize" -f json` |
| `--max-turns` | 无 | `int` | `8` | 限制 Agent Loop 最大轮次，主要用于非交互与自动模式。 | `bytemind -p "fix tests" --max-turns 5` |
| `--auto-accept` | 无 | `bool` | `false` | 自动接受允许的编辑和命令执行，适用于可信环境。 | `bytemind -p "run tests" --auto-accept` |
| `--version` | `-v` | `bool` | `false` | 输出版本、构建时间和 commit hash。 | `bytemind --version` |
| `--help` | `-h` | `bool` | `false` | 输出帮助信息。 | `bytemind --help` |

### 5.3 根命令行为

| 命令 | 说明 | 示例 |
| --- | --- | --- |
| `bytemind` | 启动交互式 TUI，会加载配置、`BYTEMIND.md`、最近会话信息。 | `bytemind` |
| `bytemind -p "query"` | 执行单轮或受限多轮任务后退出。 | `bytemind -p "解释为什么测试失败"` |
| `bytemind -p "query" -f json` | 用 JSON 输出结果，适合脚本与 CI。 | `bytemind -p "summarize diff" -f json` |
| `bytemind -m <model>` | 指定模型启动交互式或非交互式会话。 | `bytemind -m qwen-max` |
| `bytemind -c <dir>` | 在指定目录内运行，覆盖当前 shell 目录。 | `bytemind -c ./backend` |

### 5.4 `config` 子命令

| 子命令 | 缩写 | 参数 | 默认值 | 功能说明 | 使用示例 |
| --- | --- | --- | --- | --- | --- |
| `bytemind config` | 无 | 无 | 打开说明输出 | 显示配置文件路径、当前生效来源与可用子命令。 | `bytemind config` |
| `bytemind config init` | 无 | `--force bool` | `false` | 生成默认 `~/.config/bytemind/config.yaml`。 | `bytemind config init` |
| `bytemind config show` | 无 | `--format text/json` | `text` | 展示合并后的生效配置，敏感字段脱敏。 | `bytemind config show --format json` |
| `bytemind config edit` | 无 | 无 | 系统默认编辑器 | 用 `$EDITOR` 打开配置文件。 | `bytemind config edit` |

### 5.5 `auth` 子命令

| 子命令 | 缩写 | 参数 | 默认值 | 功能说明 | 使用示例 |
| --- | --- | --- | --- | --- | --- |
| `bytemind auth` | 无 | 无 | 显示认证帮助 | 显示已配置 provider、认证来源与可用操作。 | `bytemind auth` |
| `bytemind auth login` | 无 | `--provider string` | 空 | 为指定 provider 写入 API Key 到本地配置或系统 keychain。 | `bytemind auth login --provider openai` |
| `bytemind auth list` | 无 | 无 | 空 | 列出已配置 provider，key 值只显示脱敏摘要。 | `bytemind auth list` |
| `bytemind auth logout` | 无 | `--provider string` | 空 | 移除指定 provider 的本地认证信息。 | `bytemind auth logout --provider anthropic` |

### 5.6 交互式会话内 slash commands

| 命令 | 参数 | 功能说明 | 使用示例 |
| --- | --- | --- | --- |
| `/help` | 无 | 显示会话命令列表。 | `/help` |
| `/clear` | 无 | 清空当前会话上下文。 | `/clear` |
| `/model <name>` | 模型名 | 切换当前模型。 | `/model deepseek-chat` |
| `/undo` | 无 | 回滚最近一次 AI 变更。 | `/undo` |
| `/quit` `/exit` | 无 | 退出程序并保存会话。 | `/exit` |
| `/compact` | 无 | 手动压缩上下文。 | `/compact` |

## 6. 数据模型

以下为 Go struct 伪代码，字段名是建议实现形态，允许在编码阶段细化。

```go
package domain

import "time"

type SessionMode string

const (
    ModeInteractive SessionMode = "interactive"
    ModePrompt      SessionMode = "prompt"
    ModeAuto        SessionMode = "auto"
)

type ApprovalMode string

const (
    ApprovalAsk        ApprovalMode = "ask"
    ApprovalAutoAccept ApprovalMode = "auto_accept"
)

type MessageRole string

const (
    RoleSystem    MessageRole = "system"
    RoleUser      MessageRole = "user"
    RoleAssistant MessageRole = "assistant"
    RoleTool      MessageRole = "tool"
)

type Session struct {
    ID             string
    CWD            string
    Mode           SessionMode
    Model          string
    Provider       string
    ApprovalMode   ApprovalMode
    Messages       []Message
    Checkpoints    []Checkpoint
    LastCompactAt  *time.Time
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

type Checkpoint struct {
    ID           string
    SessionID    string
    MessageIndex int
    Summary      string
    FileChanges  []FileChange
    CreatedAt    time.Time
}

type Message struct {
    ID          string
    SessionID   string
    Role        MessageRole
    Content     string
    ToolCalls   []ToolCall
    ToolResults []ToolResult
    TokenUsage  TokenUsage
    CreatedAt   time.Time
}

type ToolCall struct {
    ID          string
    Name        string
    InputJSON   string
    RiskLevel   string
    RequiresAck bool
    StartedAt   *time.Time
    FinishedAt  *time.Time
}

type ToolResult struct {
    ToolCallID string
    Success    bool
    Output     string
    Error      string
    ExitCode   *int
    Metadata   map[string]string
}

type FileChange struct {
    Path         string
    ChangeType   string
    BeforeSHA256 string
    AfterSHA256  string
    Diff         string
}

type TokenUsage struct {
    InputTokens  int
    OutputTokens int
    TotalTokens  int
}

type Provider interface {
    Name() string
    SupportsTools() bool
    SupportsStreaming() bool
    SupportsReasoning() bool
    Chat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
}

type ChatRequest struct {
    Model       string
    Messages    []Message
    Tools       []ToolSpec
    MaxTokens   int
    Temperature float64
    Metadata    map[string]string
}

type StreamEvent struct {
    Type      string
    TextDelta string
    ToolCall  *ToolCall
    Usage     *TokenUsage
    Done      bool
}

type Tool interface {
    Name() string
    Description() string
    InputSchema() any
    RequiresApproval(input map[string]any) bool
    Execute(ctx context.Context, call ToolCall, env ToolEnv) (ToolResult, error)
}

type ToolEnv struct {
    CWD          string
    ApprovalMode ApprovalMode
    SessionID    string
    Timeout      time.Duration
}

type Config struct {
    DefaultModel  string
    ApprovalMode  ApprovalMode
    MaxTurns      int
    Providers     map[string]ProviderConfig
    ProjectPrompt string
    Shell         ShellConfig
    Git           GitConfig
    UI            UIConfig
}

type ProviderConfig struct {
    Type        string
    BaseURL     string
    APIKey      string
    Model       string
    MaxTokens   int
    TimeoutSec  int
    Temperature float64
}

type ShellConfig struct {
    AllowPrefixes     []string
    DenyPrefixes      []string
    DefaultTimeoutSec int
}

type GitConfig struct {
    AutoCommit bool
    CoAuthor   string
}

type UIConfig struct {
    Theme        string
    ShowTokens   bool
    StreamOutput bool
}
```

## 7. 错误处理策略

| 场景 | 用户可见行为 | 系统处理策略 | 非交互退出码建议 |
| --- | --- | --- | --- |
| LLM API 超时 | 提示“模型请求超时”，展示 provider、模型和耗时 | 自动重试最多 3 次，指数退避；最后一次失败后保留会话并允许用户重试 | `20` |
| LLM API 限流 | 提示“请求被限流”，显示重试建议 | 读取 `Retry-After` 或采用退避；非交互模式返回结构化错误 | `21` |
| LLM API 余额不足 | 提示“账户额度不足或服务不可用” | 不重试，建议切换模型或 provider | `22` |
| 无效 API Key | 提示“认证失败”，指出对应 provider | 不重试；引导使用 `bytemind auth login` 或环境变量修复 | `23` |
| 文件权限不足 | 提示具体路径与系统错误 | 保留原文件，不生成部分写入；会话中记录失败的 `ToolResult` | `30` |
| 文件不存在 | 明确指出路径不存在；若是创建型操作则询问是否新建 | 读取失败不重试；写入场景可转为新建候选 diff | `31` |
| 磁盘满 | 提示写入失败和磁盘状态 | 停止后续写文件操作；保留 diff 供用户稍后重试 | `32` |
| Shell 非零退出码 | 显示退出码、stderr 摘要、失败命令 | 不视为系统崩溃；将结果回流模型供下一轮分析 | `40` |
| Shell 超时 | 标记状态为 `timeout` | 发送中断信号，必要时强制 kill；保留已有输出 | `41` |
| 网络断开 | 提示网络不可达 | 若在流式过程中断，则保留已接收内容和会话；允许继续重试 | `24` |
| 工具调用参数错误 | 显示“模型生成了无效工具参数” | 生成结构化 `tool_error` 回流模型，允许模型自我修正下一轮调用 | `50` |
| 上下文超出模型窗口 | 提示上下文过长并建议 `/compact` | 自动做摘要压缩；若压缩仍失败，则要求用户缩小任务范围 | `25` |

### 7.1 统一处理原则

1. 任何错误都必须区分“模型错误”“工具错误”“系统错误”“用户输入错误”。
2. 已确认的文件变更不得因为后续命令失败而回滚，除非用户显式执行 `/undo`。
3. 非交互模式的 stderr 只输出错误摘要，stdout 严格保留给业务结果。
4. 结构化输出模式下，错误必须带 `code`、`message`、`retryable` 三个字段。

## 8. 系统 Prompt 设计

### 8.1 Prompt 组装顺序

ByteMind 在每轮请求前按以下顺序组装最终提示：

1. **基础身份与能力描述**
2. **安全规则与输出格式约束**
3. **可用工具列表与 JSON schema**
4. **项目指令文件 `BYTEMIND.md`**
5. **当前会话摘要或 compact 后摘要**
6. **本轮用户指令**

### 8.2 基础身份与能力描述

系统提示的基础段应明确：

1. ByteMind 是运行在本地终端内的 AI Coding Agent。
2. 它可以读取文件、搜索代码、编辑文件、执行 Shell 命令。
3. 所有写操作和命令执行都必须遵守权限策略。
4. 它的目标是帮助用户完成开发任务，而不是脱离上下文进行泛化回答。

### 8.3 可用工具列表的注入位置

- 工具列表应作为 system prompt 的结构化段落插入在安全规则之后。
- 每个工具包括：名称、用途、输入字段、约束、是否可能触发审批。
- 对 Anthropic provider，工具列表转为原生 tool schema；对 OpenAI-compatible provider，转为 function/tool schema；同时保留一份人类可读描述用于提示模型如何选择工具。

### 8.4 项目指令文件 `BYTEMIND.md` 的注入位置

- `BYTEMIND.md` 在工具列表之后、会话摘要之前注入。
- 注入策略为“原文优先、超长时摘要”。
- 仅加载工作目录根目录下的 `BYTEMIND.md`，MVP 不做多层级递归继承。
- 若文件不存在，则该段为空，不报错。

### 8.5 用户指令的注入位置

- 用户本轮输入始终作为最后一个用户消息注入。
- 非交互模式下，stdin 内容作为附加上下文，放在主 prompt 之前，以引用块或结构化附件形式插入。

### 8.6 输出格式约束

系统 prompt 应约束模型：

1. 先思考任务是否需要工具，避免无意义调用。
2. 需要事实时优先读文件、搜索，不要猜测项目实现。
3. 若要修改代码，应输出结构化工具调用，而不是在自然语言中直接粘贴完整文件。
4. 非交互模式指定 `json` 时，只能输出 JSON，不得输出 Markdown 包装。
5. 回答应简洁、技术友好、基于真实文件路径和命令结果。

### 8.7 安全规则

系统 prompt 必须显式写明：

1. 不泄露 API Key、环境变量密钥、token、cookie 等敏感信息。
2. 未经确认，不执行危险命令，不写文件，不越过工作目录边界。
3. 不读取 `.env`、私钥、证书等敏感文件，除非用户明确要求且权限允许。
4. 如果工具参数不完整或超出边界，应先返回错误而不是自行猜测补全。
5. 如果上下文不足，应先搜索或读取文件。

### 8.8 Prompt 模板框架

```text
[System: Identity]
你是 ByteMind，一款运行在本地终端中的 AI Coding CLI...

[System: Safety]
- 不泄露任何 API Key 或敏感配置
- 未确认不得写文件或执行命令
- 所有操作限制在工作目录内

[System: Tools]
- file_read(...)
- file_edit(...)
- shell(...)

[System: Project Instructions]
<BYTEMIND.md 内容>

[System: Session Summary]
<compact summary if any>

[User]
<当前用户问题>
```

## 9. 项目目录结构

### 9.1 MVP 精简目录结构

```text
bytemind/
├── cmd/bytemind/main.go
├── internal/
│   ├── agent/          # Agent Loop
│   ├── provider/       # LLM Provider
│   ├── tools/          # 内置工具
│   ├── tui/            # TUI 界面
│   ├── config/         # 配置
│   ├── permission/     # 权限
│   ├── session/        # 会话
│   └── git/            # Git 集成
├── go.mod
├── Makefile
└── README.md
```

### 9.2 目录职责说明

| 目录/文件 | 职责 | 核心文件建议 | 依赖关系 |
| --- | --- | --- | --- |
| `cmd/bytemind/main.go` | CLI 入口，解析 flags、初始化依赖、启动 TUI 或非交互模式 | `main.go` | 依赖 `config`、`agent`、`tui`、`session` |
| `internal/agent/` | 实现 Agent Loop、消息编排、工具调用调度、compact 触发 | `loop.go`、`runner.go`、`prompt.go` | 依赖 `provider`、`tools`、`permission`、`session` |
| `internal/provider/` | 封装 OpenAI-compatible 和 Anthropic provider，统一 streaming 和 tool calling | `provider.go`、`openai.go`、`anthropic.go` | 依赖 `config` |
| `internal/tools/` | 内置工具注册、schema 定义、文件与 shell 工具执行 | `registry.go`、`file_read.go`、`file_edit.go`、`shell.go` | 依赖 `permission`、`session` |
| `internal/tui/` | Bubble Tea Model、消息列表、输入框、状态栏、diff 视图 | `model.go`、`view.go`、`keymap.go` | 依赖 `agent`、`session` |
| `internal/config/` | 读取 YAML、环境变量覆盖、默认值、`BYTEMIND.md` 加载 | `config.go`、`loader.go` | 低层模块，被所有上层依赖 |
| `internal/permission/` | 文件写入确认、命令确认、路径校验、审批模式 | `policy.go`、`approval.go` | 依赖 `config` |
| `internal/session/` | 会话持久化、消息历史、checkpoint、恢复与 compact | `store.go`、`session.go`、`checkpoint.go` | 依赖 `config` |
| `internal/git/` | Git 状态检查、自动 commit、`/undo`、脏文件保护 | `git.go`、`undo.go` | 依赖 `session`、`permission` |
| `go.mod` | Go 模块依赖定义 | `go.mod` | 全局依赖描述 |
| `Makefile` | 构建、测试、release 打包命令 | `Makefile` | 驱动开发与 CI |
| `README.md` | 安装、配置、CLI 示例、权限模型说明 | `README.md` | 面向用户文档 |

### 9.3 依赖原则

1. `config`、`session` 属于底层基础模块，不依赖 `tui`。
2. `agent` 只通过接口依赖 `provider` 和 `tools`，不依赖具体实现。
3. `tui` 不直接操作文件系统或 Git，必须经由 `agent` 发起请求。
4. `git` 不参与模型推理，仅负责状态检查和回滚实现。

## 10. MVP 里程碑与验收标准

### 10.1 M1：骨架搭建

- **目标描述**：完成 CLI 启动、基础 TUI、单模型对话和基本会话生命周期。
- **包含的功能项**：FR-002、FR-004、FR-030、FR-032、FR-033、FR-060、FR-062、FR-063、FR-080、FR-084
- **Demo Scenario**
1. 在空项目目录执行 `bytemind`。
2. TUI 成功启动，状态栏显示模型与工作目录。
3. 输入“你好，请说明你能做什么”。
4. 模型流式回复。
5. 执行 `/help` 查看命令。
6. 执行 `/exit` 退出并再次启动。
- **期望结果**：程序能稳定启动、响应、退出；配置文件可生效；会话可被保存。
- **依赖关系**：无，作为基础里程碑。

### 10.2 M2：工具系统

- **目标描述**：打通 Agent Loop、文件工具、搜索工具、Shell 工具和权限确认。
- **包含的功能项**：FR-001、FR-003、FR-020、FR-021、FR-022、FR-023、FR-024、FR-025、FR-026、FR-034、FR-040、FR-041、FR-043
- **Demo Scenario**
1. 在一个 Go 项目中输入“找到 HTTP server 的入口并解释调用链”。
2. 观察 `grep`、`file_read` 状态展示。
3. 输入“把默认端口从 8080 改成 9090”。
4. 查看 diff，确认写入。
5. 再输入“运行单元测试”。
6. 在命令确认框中选择允许。
- **期望结果**：工具调用能闭环执行；文件写入和命令执行都有确认；输出结果回流会话。
- **依赖关系**：依赖 M1 的基础会话和 UI。

### 10.3 M3：开发者体验

- **目标描述**：补齐 Git 闭环、脏文件保护、模型切换、项目指令文件和上下文压缩。
- **包含的功能项**：FR-005、FR-012、FR-035、FR-050、FR-051、FR-052、FR-053、FR-061、FR-081、FR-082、FR-083、FR-085
- **Demo Scenario**
1. 在 Git 仓库内创建一个 bugfix 任务。
2. 让 ByteMind 修改代码并确认写入。
3. 自动生成 commit。
4. 执行 `/undo` 回滚。
5. 添加 `BYTEMIND.md`，重新启动会话，验证项目规则生效。
6. 切换模型后继续对话，再执行 `/compact`。
- **期望结果**：AI 改动可提交可撤销；脏文件不被覆盖；项目指令自动注入；上下文可压缩。
- **依赖关系**：依赖 M2 的工具和权限系统。

### 10.4 M4：多模型与非交互

- **目标描述**：完成 OpenAI-compatible、Anthropic、JSON 输出、stdin、最大轮次等自动化能力。
- **包含的功能项**：FR-010、FR-011、FR-013、FR-042、FR-070、FR-071、FR-072、FR-073
- **Demo Scenario**
1. 设置两个 provider 的配置。
2. 运行 `bytemind -m deepseek-chat -p "解释这个仓库" -f json`。
3. 运行 `cat panic.log | bytemind -m claude-3-7-sonnet -p "总结错误" --max-turns 3`。
4. 在无 TTY 环境中执行一个需要权限的任务。
- **期望结果**：不同 provider 工作正常；JSON 输出稳定；stdin 注入正确；需要人工确认的任务在无 TTY 下安全失败。
- **依赖关系**：依赖 M1-M3 的核心执行能力。

### 10.5 M5：打磨发布

- **目标描述**：完成跨平台构建、安装分发、文档、可靠性与性能优化。
- **包含的功能项**：NFR-001 至 NFR-042 全量验证，重点覆盖 NFR-020、NFR-021、NFR-022、NFR-023
- **Demo Scenario**
1. 在 CI 中构建 macOS、Linux、Windows 二进制。
2. 使用 `go install` 与安装脚本完成安装。
3. 执行 smoke test：启动、文件读取、非交互回答、写入确认、Shell 中断。
4. 比较冷启动与内存指标。
- **期望结果**：构建和安装链路稳定；核心 NFR 满足目标；README 可指导首次使用。
- **依赖关系**：依赖前四个里程碑完成。

## 11. 风险与依赖

| 风险/依赖 | 说明 | 影响 | 缓解策略 |
| --- | --- | --- | --- |
| Go Bubble Tea 生态成熟度风险 | Bubble Tea 适合 TUI，但复杂 diff、长日志折叠、跨平台键位兼容需要额外工程投入。 | 可能影响 TUI 交互细节和发布节奏。 | 先做简单稳定的消息流 + 状态栏，复杂布局延后；核心逻辑与 TUI 解耦。 |
| LLM API 兼容性 | 不同 Provider 在 tool calling、流式 chunk 格式、token 统计、错误码上差异明显。 | 同一 Agent Loop 在不同模型上行为不一致。 | 建立统一 `Provider` 适配层和 capability flags，先验证 OpenAI-compatible + Anthropic 两条主链。 |
| ripgrep 外部依赖 | `grep` 能力如果依赖外部 `rg`，会与“单二进制、零依赖”定位冲突。 | 安装体验和跨平台一致性受影响。 | MVP 设计为“优先调用 `rg`，缺失时回退 Go 原生扫描”；后续评估嵌入式分发。 |
| tree-sitter Go 绑定成熟度 | 若 MVP 过早引入 tree-sitter 做符号级解析，可能增加跨平台构建复杂度。 | 拉长开发周期，并增加二进制大小与维护成本。 | MVP 不将 tree-sitter 设为必选，优先 `grep + file_read`，LSP 作为增强项。 |
| Git 命令可用性 | 用户环境可能不存在 Git，或仓库处于 detached/headless 状态。 | 自动 commit、`/undo`、脏文件保护无法完整工作。 | Git 能力做可选降级；无 Git 时仍保留会话级 diff 回滚。 |
| 国产模型稳定性 | 部分国产 OpenAI-compatible 服务在工具调用与 JSON 输出上兼容性不完全。 | 影响中国市场主打卖点。 | 优先测试 DeepSeek、Qwen、GLM 等主流服务，建立 provider 兼容矩阵。 |
| Windows Shell 差异 | Windows 默认 `powershell`/`cmd` 与 Linux/macOS shell 行为不同。 | 命令执行、超时、中断语义不同。 | 为各平台封装统一 shell runner，并在配置中显式定义 shell 类型。 |

## 12. 开放问题（Open Questions）

1. 项目指令文件命名是否长期固定为 `BYTEMIND.md`，还是在 MVP 后兼容 `.bytemind.md`、`bytemind.toml`、`AGENTS.md`？
2. 是否在 MVP 阶段支持 MCP Client，还是严格留到下一个版本以换取更快发布？
3. 默认模型选择策略是什么：国产模型优先、用户配置优先，还是按 provider 能力自动推断？
4. 是否需要在 MVP 阶段支持 Vim 模式输入，还是先以标准多行输入为主？
5. 会话数据存储位置和格式最终选择什么：SQLite、JSONL 还是混合方案？
6. 是否需要在 MVP 阶段默认开启自动上下文压缩，还是只提供手动 `/compact`？
7. `auto-accept` 是否允许在非交互模式下默认执行 Shell 命令，还是仅允许文件写入？
8. `grep` 是否应以外部 `ripgrep` 为硬依赖，还是接受性能稍弱的内置 fallback？
9. Git 自动 commit 是否默认开启，还是作为项目级配置项显式启用？
10. `BYTEMIND.md` 是否只在根目录查找，还是允许后续版本增加子目录继承规则？
