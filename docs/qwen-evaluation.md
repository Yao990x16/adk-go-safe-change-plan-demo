# Qwen 真实验收与延迟优化

2026-11-21 在同一 Qwen 服务上，使用相同场景、工具、提示词和模型参数顺序运行两组验收。改动发生在最后一个必需工具结果返回之后。Go 已经拥有生成报告所需的信息，可以在本地结束采集。

| 场景 | 优化前 | 优化后 | 模型调用 | 基线最后一轮 |
| --- | ---: | ---: | ---: | ---: |
| `checkout-api` CPU 高且 CrashLoop | 39.560 s | 10.064 s | 3 → 2 | 29.430 s |
| `image-worker` 队列积压 | 35.308 s | 6.702 s | 3 → 2 | 28.728 s |

表中为每个场景的一次前后实测。服务负载和模型输出长度会影响秒数。CPU 场景中 Qwen 将两个 Runbook 调用放在同一轮。如果模型分两轮请求这两个工具，同样会省掉证据齐全后的那一轮。

## 复现

在 shell 环境中设置 `QWEN_BASE_URL` 和 `QWEN_MODEL`，分别填写以 `/v1` 结尾的服务地址与模型完整 ID。每条命令都会按顺序执行两个虚构场景。

```bash
make eval-qwen-baseline
make eval-qwen
```

命令通过 `-count=1` 禁用 Go 测试结果缓存。`TestQwenLive` 的 `-qwen-live` 开关默认关闭，普通 `go test ./...` 跳过这个测试。真实验收会发出网络请求，端点配置来自环境变量。

每个用例输出 `wall`（从启动 Agent 到完成报告的耗时）、`model_calls`（实际模型适配器调用次数）和 `model_ms`（每次调用耗时，单位毫秒）。本地回调生成的结束响应不计入模型调用。耗时用于比较，断言使用调用次数和业务结果。CLI 的 `-trace` 也会输出这些指标，其中总耗时用 `wall_ms` 表达。

## 实现拆解

1. **先固定验收条件。** 用 `toolSelectionCase` 解码已有 JSON 用例，再用共享的 `assertToolSelection` 校验调用参数和报告。它比较工具调用集合，然后单独验证第一步为服务检查，允许 Qwen 以任意顺序读取所需 Runbook。每条计划均须带回滚条件和回滚方式。适合从 `agent_test.go` 学习 Go 结构体、JSON 标签、`t.Helper` 和子测试。
2. **在模型入口计时。** `measuredModel` 内嵌 `model.LLM`，复用其 `Name` 并覆盖 `GenerateContent`。用 `time.Now` 和 `time.Since` 记录响应迭代器运行的耗时。计时写在迭代器函数内部，因为拿到迭代器时模型请求通常还未发出。基线结果显示两条案例的最后一轮各耗费约 29 秒。
3. **从实际工具结果决定何时结束。** `BeforeToolCallback` 检查拟执行的调用，`AfterToolCallback` 保存完成的结果。`BeforeModelCallback` 在下一次推理前检查是否已完成成功的服务检查、是否已读齐 `requiredTopics`、是否有策略或工具错误。证据还缺时返回 `nil, nil`，ADK 继续调用模型。证据齐全时返回本地结束响应，`renderReport` 继续校验证据和生成报告。
4. **验证少一次请求且行为一致。** `TestQwenToolRoundTrip` 用 `httptest` 对比 HTTP 请求数 3 → 2 并逐字比较报告。CPU 回归测试把两份 Runbook 分成两轮，验证只读到一份时会继续采集。另一个用例模拟模型提前宣告完成，检查缺少 Runbook 时不会生成报告。最后用真实服务跑同样的业务断言。

一个练习是按顺序看 `TestFinishWithEvidenceWaitsForAllRunbooks`、`finishWhenReady`、`requiredTopics`。先预测每轮结束时证据表有哪些条目，再运行 `go test -run '^TestFinishWithEvidence' -v .` 观察调用次数和缺证据分支。
