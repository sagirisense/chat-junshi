# Go 迁移状态与验收

## 当前实现（2026-10-13）

Wails 已直接调用 Go 业务服务，不再启动 Python 子进程或使用 Python JSONL 中转。React 保留原操作协议，安装包不包含 Python 运行时或脚本。旧 Python 实现和测试保留作兼容性对照。

| 模块 | 当前实现 |
| --- | --- |
| 聊天与证据 | internal/evidence：时间标签、纠错、稳定消息 ID、版本1 JSON、对象隔离和导入导出 |
| 关系记忆 | internal/memory：兼容旧 SQLite 表、同意、暂停、撤销、配额、遗忘与删除 |
| 业务状态 | internal/service：身份核对、任务取消、过期结果丢弃、档案和受控记忆；首次打开旧数据前备份聊天及一致 SQLite 快照 |
| 模型与资讯 | 三协议 API、Codex 临时会话及通信复用、流式预览、工具请求拒绝；RSS 缓存与日期/来源校验 |
| 采集 | internal/capture + native/wgc：精确 HWND/PID/创建时间校验、无激活还原、首帧重试、重连与关闭 |
| OCR | internal/ocr：同一 PP-OCRv4 模型、气泡合并、截断保护、时间关联及按出现顺序去重 |
| 安装 | PowerShell + Wails + Inno Setup：固定资产哈希、运行文件白名单、依赖许可与文件清单 |

构建需要 Go 1.25+、CGo/MinGW、Rust GNU、现代 Node 和 Inno Setup。运行不需要这些工具，仍依赖 WebView2、WGC/ONNX DLL、VC 运行库和模型，见 [构建说明](../packaging/README.md)。

## 来源与兼容性

Go 检测代码改编自 Apache-2.0 的 [gopaddleocr](https://github.com/multippt/gopaddleocr)，固定版本 d4b7cc8b442974de8020337bb58495bef1c4efe0，调整配置读取、缩放和膨胀。沿用 v4 模型，字典从模型 metadata 提取；ppocr_keys_v5.txt 是参考识别器的文件名，并不表示升级到 v5。

WGC 使用 MIT 的 windows-capture 2.0.1 编译薄 Rust DLL。SQLite 固定 modernc.org/sqlite 1.39.1。完整许可与上游版权见 [NOTICE](../NOTICE) 和 [licenses](../licenses/)。

## 已验证与待验收

- Go 测试覆盖聊天、身份、隔离、取消、三协议模拟服务、CLI 合成通信、记忆及备份。虚构旧库通过 Go/Python 双向读写和撤销对照。
- 长文本相似度与 Python SequenceMatcher 高频字符规则完成 140 组固定随机样本对照。
- 原生 WGC 虚构窗口测试通过首帧、尺寸变化、重复关闭及销毁后的身份拒绝；虚构像素覆盖多行气泡、半截保护、重复回复和历史滚动。
- 同一 v4 模型的三行虚构中文与 Python 文本一致。原型单次加载约 0.645 秒、识别约 0.065 秒，仅为该样本记录，不能推断真实微信准确率或全链路性能。
- Vite、Wails 和 Inno 安装包已构建，运行清单排除 Python、旧服务和私有数据。

真实微信 GUI、不同 DPI/遮挡/最小化、模型供应商联调和旧安装升级仍需人工验收，由用户自行测试。编译、虚构窗口及模拟协议通过不代表这些场景完成。

人工验收需核对首帧及首批消息耗时、原文/说话人/时间和重复回复完整性；回复、润色、话题和导入导出正常；旧数据备份可读；对象隔离、同意和撤销有效；升级后不启动旧 Python 服务。仅自行复制发送，不自动发送微信消息。

## 提交边界

提交代码、测试、文档和许可文本。模型、DLL、构建产物、聊天、截图、关系数据库、密钥和登录凭据不进入 Git。
