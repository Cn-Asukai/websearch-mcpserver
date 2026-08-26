# T12 Load() 用 applyKnownEnv 回填环境变量

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P2-7
- 优先级：P2
- 状态：完成
- 依赖：可与 [T04](T04-explicit-preset-config.md) 同 PR（都碰配置加载，无硬依赖）
- 方案：[plans/wave-3-p2.md](../plans/wave-3-p2.md)

## 背景（部分已修）

`applyKnownEnv` 已存在且 `Default()` 会调用。`Load()` Unmarshal 后没有调用 → 精简 yaml 缺 `tavily.api_key` 等字段时 `TAVILY_SK` 仍可能无效。

## 要改什么

`pkg/config/config.go` `Load()`：默认值处理结束后 `applyKnownEnv(&conf)`。

测试：临时 yaml 仅 `port: 8338`，`t.Setenv("TAVILY_SK", "x")`，`Load` 后 `APIKey == "x"`。覆盖 BAIDU_SK / EXA_API_KEY / LLM_* / MINERU_TOKEN 至少各抽一两个即可。

## 验收

- `go test ./pkg/config/...`
- 有完整 yaml 字段时，yaml 值仍可被同名 env 覆盖（与现有 `applyKnownEnv` 语义一致：env 非空则覆盖）
