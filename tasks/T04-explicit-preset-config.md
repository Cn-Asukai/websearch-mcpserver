# T04 首次启动写出可编辑的预设 config.yaml

- Issue：[#2](https://github.com/daidaiJ/websearch-mcpserver/issues/2) P1-2
- 优先级：P1
- 状态：完成
- 依赖：无
- 方案：[plans/wave-2-p1.md](../plans/wave-2-p1.md)

## 产品意图（不要做成 LoadOrDefault）

「零配置」指 **install / init / 首次 start 生成一份与 example 相同的 `config.yaml`**，用户有显式可改的文件。不是进程内隐式 `Default()`。

Windows `install` 已经在 exe 旁写 `config.ExampleConfig`（`cmd/setup_windows.go:200-210`）；CLI `init` 同样。daemon `start` 缺文件就退出，文档却写成直接 `start`。

**禁止** 把 `cmd/main.go` 改成 `config.LoadOrDefault`。

## 要改什么

1. `pkg/config` 抽出 `EnsureExampleFile(path string) (created bool, err error)`：不存在则写 `ExampleConfig`，存在则不动。
2. `install` 与 `cli init` 改用该函数（行为保持幂等）。
3. `start`：没有 `-c` / `WEBSEARCH_CONFIG`、且默认搜索路径找不到文件时，向可执行文件目录写入 `config.yaml`，stdout 提示路径，再 `Load`。显式路径缺失仍报错退出。
4. README / `docs/installation.md`（及英文）：说明「无需手写配置」= 生成预设 yaml；改配置就改这份文件。不要承诺「没有任何 yaml、靠隐式默认值运行」。

## 验收

- `go test ./pkg/config/... ./cmd/cli/...`
- 空目录首次 `start`（无 `-c`）会在 exe 旁出现 `config.yaml`，内容与 example 一致，且随后能 Load
- 已有 `config.yaml` 再 `install`/`init`/`start` 不覆盖用户改过的内容
- 文档不再把「零配置」写成纯隐式默认值
