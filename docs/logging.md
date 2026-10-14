# 本地诊断日志

Go/Wails 桌面端启动后自动写入数据根目录下的 `data/logs/desktop.log`（每行一条 JSON）。单文件最大 2 MiB，轮转后保留 `desktop.log.1`，最多约 4 MiB。

- 源码运行：项目目录下的 `data/logs/`。
- 安装版：`%LOCALAPPDATA%/ChatJunshi/data/logs/`。
- 设置了 `JUNSHI_DATA_ROOT`：该目录下的 `data/logs/`。

排查时先复现错误，再查看相应时间的 `level=error` 记录。`stage` 表示失败阶段，`elapsed_ms` 表示耗时，`error_type` 和 `callers` 表示错误类型及处理错误时的代码位置（不是原始异常堆栈）。异步分析的失败记录可能出现在操作调用成功之后。

日志覆盖启动、主要操作调用、模型生成、结果校验、旧结果丢弃、采集故障、记忆写入和关闭。不会记录请求载荷、模型响应、密钥、聊天标题、聊天内容、关系档案或原始错误文本。未识别的操作名统一记为 `app.call.other`。界面仍显示原有错误提示；反馈问题时可同时说明提示与复现步骤。

日志属于本机运行数据，已被现有 Git 忽略规则排除。此机制用于 Go 桌面端；旧 Python 入口及其采集诊断脚本已移除。
