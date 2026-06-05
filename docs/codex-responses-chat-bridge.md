# Codex Responses 与 OpenAI Chat 协议互转注意点

> 受影响代码：`relay/adaptor/codex/responses_to_chat.go`、`relay/adaptor/codex/chat_to_responses.go`、`relay/adaptor/codex/utils.go`、`relay/adaptor/codex/handler.go`
> 关联样本：`relay/adaptor/codex/request_raw.txt`、`relay/adaptor/codex/response_raw.txt`、`relay/adaptor/codex/response_codex_raw.txt`

## 1. 背景

Codex CLI 走的是 OpenAI 新的 **Responses** 协议；上游多数模型只能消费老版 **Chat Completions** 协议（含流式 SSE）。`relay/adaptor/codex` 适配器在两侧做翻译：

- **请求方向（Responses → Chat）**：把 Codex CLI 发来的 Responses 请求体转成上游能懂的 Chat Completions 请求。
- **响应方向（Chat → Responses）**：把上游返回的 Chat SSE 流，按 Responses 协议的 event 序列重新组装，再喂回给 Codex CLI。

两边协议在 tool / function_call / 流式事件上形状差异很大，特别是内置的 `tool_search` / `web_search` 与 **deferred namespace tool** 这两类客户端侧工具。下面是逐项要点与踩过的坑。

## 2. 两个方向的互转职责

### 2.1 Responses → Chat（请求方向，`responses_to_chat.go`）

| 维度 | Responses 侧 | Chat 侧输出 |
|------|--------------|--------------|
| messages | `input: []`（含 `message` / `function_call` / `function_call_output` / `reasoning` / `tool_search_*` 等 item） | `messages: []`（role ∈ `system` / `user` / `assistant` / `tool`） |
| tools | `tools: []`（`type` 可能是 `function` / `tool_search` / `web_search` / `namespace` / `custom`） | 统一为 `type:"function"`，namespace 子工具扁平化为 `namespace__name` |
| builtin | `tool_search` / `web_search`（`execution:"client"`） | 仍以 `function` 名 `tool_search` / `web_search` 出现，把 `execution:"client"` 这种语义**留到 Chat→Responses 侧还原** |
| tool result | `input: [{type:"function_call_output", call_id, output}]` 以及 `tool_search_output` / `web_search_output` | 收敛为 `role:"tool"` 的 message，保留 `tool_call_id`（参见 §4） |
| namespace tools | 客户端**不发**到顶层 tools，而是 `tool_search_output.tools` 里延迟出现 | 合并到 Chat 顶层 `tools` 中，命名按 `__` 扁平化（参见 §5） |

### 2.2 Chat → Responses（响应方向，`chat_to_responses.go`）

| 维度 | Chat 侧输入 | Responses 侧输出 |
|------|------------|------------------|
| stream events | `delta.content` / `delta.reasoning_content` / `delta.tool_calls[*]` / `finish_reason` | `response.output_item.added` → `*.delta` / `response.reasoning_*` / `response.output_item.done` / `response.completed` |
| reasoning | `delta.reasoning_content` 文本 | `response.reasoning_summary_text.delta` + 同 id 的 `.done` + `output_item.added` / `.done` |
| content | `delta.content` 文本 | `response.output_text.delta` + 同 id 的 `.done` + `message` item |
| tool_calls | `delta.tool_calls[*].function.{name, arguments}` 累积 | `function_call` item（含 `name` / `arguments` / `call_id` / `id`），普通 function 还要发 `function_call_arguments.delta` |
| custom tool | 上游 `name="custom"` 之类的纯代理工具 | `custom_tool_call` item，**带 input 不带 arguments** |
| builtin | 上游 `name="tool_search"` / `name="web_search"` | `tool_search_call` / `web_search_call` item，**带 `execution:"client"`** + 同 id 的 lifecycle 事件 |
| namespace tool | 上游 `name="multi_agent_v1__spawn_agent"` 之类 | 反扁平化：`name:"spawn_agent"` + `namespace:"multi_agent_v1"`，type 为普通 `function_call` 或 `custom_tool_call` |

上下文（`CodexToolContext`）从**原始 Codex Responses 请求体**与 `input[].*_output.tools` 一起重建，**不能只靠 Chat tool 名推断**，否则 namespace 还原会丢。

## 3. builtin `tool_search` / `web_search` 的协议形状

Chat 协议里没有 `tool_search_call` / `web_search_call` 这种 item 类型，所以从 Chat → Responses 时必须按 Codex 原生形态重新包装。关键形状：

- **item 类型与 id 前缀**：
  - tool_search → `type:"tool_search_call"`，id 前缀 `tsc_`（例：`tsc_019e91c6...`）
  - web_search → `type:"web_search_call"`，id 前缀 `wsc_`
- **执行位置**：`execution:"client"` 必须出现在 `output_item.added` / `.done` / `response.completed` 三处 item 上，**缺一项 Codex CLI 就不认这是客户端本地执行的内置工具**。
- **arguments 形态**：
  - 合法 JSON object/array（`{"query":"..."}` / `["a","b"]`）→ 保持**结构化**（不要 stringify 成 `"{\"query\":...}"`，否则 codex 解析不到 query 字段）。
  - 不是合法 JSON（裸字符串 / 自定义 DSL）→ 保持**字符串**形态，向后兼容。
- **lifecycle / search_query 事件**必须引用同一 item id：
  ```
  response.output_item.added                (id=tsc_xxx, status="in_progress", execution="client")
  response.tool_search_call.in_progress     (item_id=tsc_xxx)
  response.tool_search_call.searching       (item_id=tsc_xxx)         ← 视实现可省
  response.tool_search_call.search_query.delta  (item_id=tsc_xxx, delta="...")
  response.tool_search_call.search_query.done   (item_id=tsc_xxx, query="...")
  response.output_item.done                 (id=tsc_xxx, status="completed", arguments=完整, execution="client")
  ```
  `web_search_*` 同形（事件名 `web_search_call.*`）。漏掉 `in_progress` / `search_query.delta` / `.done` 中的任何一个，Codex CLI 都不会本地执行 search，也不会发起下一轮请求回填 output。

## 4. tool result 闭环

Codex CLI 回填 tool result 时使用的 item 类型不只是 `tool_search_call_output` / `web_search_call_output`，**真实请求里常出现 `tool_search_output` / `web_search_output`（无 `_call_`）**——必须两种都识别。

转换规则：

- **Responses → Chat**：所有 builtin tool result item 一律收敛为 Chat 的 `role:"tool"` message：
  - `tool_call_id`：取 call item 的 id 去前缀（`tsc_xxx` → `xxx`，`wsc_xxx` → `xxx`），作为 Chat `tool_call_id`。
  - `content`：item 的 `output` / `result` 字段原样写入；如果 `output` 是字符串直接放，是对象/数组就序列化为字符串。
  - **绝不能出现"empty content 但 input 里其实有 `tools` 数组"的情况**。`tool_search_output` 经常 `output:""` 但带 `tools:[{type:"namespace",...}]`——这是把"延迟发现的子工具清单"带回给上游模型的唯一通道。必须把 `output` 之外的 payload（如 `tools` 数组）序列化为 JSON 字符串塞进 tool message 的 `content`，否则上游拿不到任何 subagent 工具信息，下一轮依然调不到。
- **Chat → Responses**：上游回的是 `role:"tool"` message，转回时按 call id 前缀还原 item 类型（`tsc_` → `tool_search_call_output`，`wsc_` → `web_search_call_output`），id 用原 call id + 前缀；content 字符串尝试反序列化为 JSON object，失败则当成纯文本。

## 5. deferred namespace tools

Codex CLI 用 **namespace + deferred discovery** 模式表达一组子工具：

- 客户端**不**把 `multi_agent_v1.spawn_agent` 这类具体工具发到顶层 `tools`，而是在 `tool_search` 被本地执行后，把 `tool_search_output.tools: [{type:"namespace", name:"multi_agent_v1", tools:[{name:"spawn_agent", ...}, ...]}]` 带回下一轮请求。
- **Responses → Chat**：
  - 把 `tool_search_output.tools` 中的 namespace 节点**合并到 Chat 顶层 `tools`**。
  - 命名按 Codex 约定扁平化：`multi_agent_v1` + `__` + `spawn_agent` → `multi_agent_v1__spawn_agent`（namespace 末尾自带 `__` 或子工具以 `__` 开头时直接拼接）。
  - 同步把这些 namespace 工具写进 CodexToolContext 的 `FunctionTools` 与 `CustomTools`，记录原始 (name, namespace) 元数据，**Chat→Responses 时反扁平化要用**。
- **Chat → Responses**：
  - 上下文**必须**从原始 Codex Responses 请求的 `tools` **加上** `input[].*_output.tools` 一起构建，单看 Chat 顶层 `tools` 会丢 namespace 元数据。
  - 上游若以 `name="multi_agent_v1__spawn_agent"` 调用，需反扁平化为 `name:"spawn_agent"` + `namespace:"multi_agent_v1"`；item 类型根据工具是否 client-executed / custom 决定（普通 function → `function_call`，custom → `custom_tool_call`）。
  - **反扁平化失败 → Codex CLI 报 `unsupported call: multi_agent_v1__spawn_agent`**，因为它的工具注册表里只有 `spawn_agent`（namespace 上下文），没有这个扁平名。

## 6. 这次踩过的坑（按阶段）

1. **builtin item 形状不像 Codex 原生**：`output_item.added` / `.done` / `response.completed` 三处 item 缺 `execution:"client"`，id 不带 `tsc_` / `wsc_` 前缀，arguments 被错误地 stringify。修：按 §3 的形状逐字段补齐，结构化 arguments 用 `sjson` 写入 object 而非字符串。
2. **lifecycle / search_query 事件缺失**：内置工具的流式转换路径上有 `continue` 提前跳过，导致 `output_item.added` 到 `.done` 之间零事件。Codex CLI 不识别这种"瞬时调用"，直接不发下一轮请求。修：拆掉那段 `continue`，按 §3 的事件序列补 `in_progress` / `searching` / `search_query.delta` / `search_query.done`。
3. **Responses → Chat 丢了真实 `tool_search_output`**：只识别 `*_call_output`，不识别 `tool_search_output` / `web_search_output`，导致上游看到的是"pending tool results / DSML"占位而非真实工具回执。修：`responses_to_chat.go` 的 `convertInputItem` / builtin output item 转换逻辑同时覆盖 `tool_search_output` / `tool_search_call_output` / `web_search_output` / `web_search_call_output`。
4. **tool_search_output.tools 没写进 Chat content，也没合并到顶层 tools**：
   - 一：转成 `role:"tool"` 时如果 `output` 字段为空就直接放过，丢掉了 `tools` payload。修：把 `output` 之外的字段（`tools` 等）序列化为 JSON 字符串塞到 tool message content。
   - 二：`tools` 数组没合并到 Chat 顶层 `tools`、也没写进 CodexToolContext 的 FunctionTools。修：抽取 namespace 节点展开，按 §5 扁平化后合并到 Chat 顶层 tools 并记录元数据。
5. **Chat → Responses 没还原 namespace**：上游调用 `name="multi_agent_v1__spawn_agent"`，转换器只把 name 字段原样透传，Codex CLI 报 `unsupported call: multi_agent_v1__spawn_agent`。修：从原始 Codex 请求 + `input[].*_output.tools` 联合构建 CodexToolContext，遇到扁平名时用 `OpenAINameForFunctionTool` 反扁平化，item 上带 `namespace` 字段。
6. **首包 tool_call 早到导致 `added.arguments` 为空**：上游首包同时带 `name` + `arguments` 时，`addToolCallItemIfNeeded` 在 `arguments` 写入缓冲区之前就发出了 `output_item.added`，导致 `added.arguments` 是 `{}` / 空串。修：保证 `addToolCallItemIfNeeded` 在首次发射时读取**当前已经写入**的 `FuncArgsBuf` 值，而不是预置 `"{}"`；若首包确实带了完整 arguments，把它原样写回 `output_item.added.arguments`。

## 7. 最终完成思路

**以原生 Codex 样本为基线，TDD 覆盖每个协议断点，逐段修复 request 和 stream 的形状，而不是改模型行为。**

具体步骤：

1. 抓三份样本：`request_raw.txt`（Codex CLI 原始 Responses 请求）、`request_conv.txt`（适配器转出的上游 Chat 请求）、`response_raw.txt`（上游 Chat SSE 回复）——三者逐字段对账，定位是哪一段丢的形状。
2. 按 §2 的职责表拆解断点（messages / tools / builtin / tool result / namespace），每个断点写一个失败用例（基于 `smartystreets/goconvey`）固化期望。
3. 按 §3 / §4 / §5 的形状规则改实现：优先用 `sjson` 做精确字段写入，避免把 object stringify 成 string；item id 一律走 `tsc_` / `wsc_` 前缀生成；lifecycle 事件按"先 added、再 in_progress、再 search_query.delta*、最后 done"的顺序补齐。
4. 跑通 `chat_to_responses_test.go` / `responses_to_chat_test.go` / `handler_test.go` 全量测试，再针对 §6 的 5 个坑做回归用例（namespace 反扁平化、tool_search_output 解析、首包 arguments 写入等）。
5. 任何"想通过修改 Codex CLI 行为绕过"的方案一律不采纳——本项目不持有 Codex CLI 代码，且客户端是固定参照面。

## 8. 验证建议

```bash
# 跑适配器全部测试（含协议形状断言）
go test -count=1 ./relay/adaptor/codex

# 只跑 namespace / builtin 相关用例
go test -count=1 ./relay/adaptor/codex -run 'TestConvertOpenAIChatToResponses_.*(Builtin|ToolSearch|WebSearch|DeferredNamespace)'
go test -count=1 ./relay/adaptor/codex -run 'TestConvertResponsesToChatRequest_.*(Builtin|Discovered|Namespace|ToolOutput)'
```

样本对账（人工 / 脚本皆可）：

- **请求形状**：`request_raw.txt` vs `request_conv.txt`——逐字段比 tools / messages，重点看 builtin 工具、namespace 工具、`tool_search_output` 是否如实展开。
- **响应形状**：用 `request_raw.txt` + `response_raw.txt` 回放 `ConvertOpenAIChatToResponsesWithContext`，逐事件检查 §3 表格中的序列是否齐全、`item_id` 引用一致、`arguments` 形态正确。
- **回归用例**：`chat_to_responses_test.go` 里 `tsc_` / `wsc_` 前缀、lifecycle 事件序列、namespace 反扁平化、tool_search_output 解析四个 Convey 块必须全绿。

出现 §6 任意一项回归时，按对应小节回查，**不要同时改多段**——逐坑收敛。
