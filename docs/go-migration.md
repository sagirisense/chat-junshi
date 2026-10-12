# 移除 Python 运行时的可行性分析

## 结论与目标

可以迁移为 **Go 桌面与业务服务 + React 前端 + 原生 OCR 运行库**，无需 Python 运行时。React/Vite 的 Node 构建工具以及 ONNX Runtime DLL/模型文件仍然需要，不能把“去 Python”理解成完全没有其他原生依赖。

目前未实施整体迁移，也未验证 Go OCR 的准确率或性能。先做独立采集/OCR 原型，再决定是否替换生产链路；保留旧实现作为功能和数据兼容性对照，不同时修改模型版本与业务规则。

## 已检查的现有依赖

| 模块 | 当前实现 | Go 替代方式 | 难度与主要风险 |
|---|---|---|---|
| 桌面宿主、剪贴板、窗口控制 | `desktop/app.go`、Wails | 已是 Go；业务迁移后移除 Python 子进程与 JSONL 中转 | 低到中：退出时取消任务、错误事件和流式状态需要兼容 |
| 微信定位与截图 | `app/capture.py`、ctypes、windows-capture | Win32 精确 HWND/PID 校验 + Windows Graphics Capture/Direct3D11 | 高：WinRT/COM 生命周期、DPI、遮挡、黑帧、最小化、窗口重建；不能仅用屏幕截图替代 |
| 中文文字检测和识别 | RapidOCR、NumPy、ONNX Runtime | Go 调用 ONNX Runtime，移植检测/识别前后处理 | 高：同一模型、字典、归一化、裁剪、排序、置信度与框坐标必须对齐 |
| 消息区、说话人、时间标签与完整性 | `app/ocr.py`、`message_times.py` | Go 图像遍历、正则、时间与消息类型 | 中到高：气泡多行合并、漏行保护、颜色分类、去重和时间推算都不能丢失 |
| 聊天存储、修正、逐条删除和导入导出 | `sessions.py`、`desktop_service.py` | Go `encoding/json`、原子文件保存 | 中：保留版本1文件、稳定消息ID、UTF-8、时间精度和选择状态；不重编号旧消息 |
| 档案、同意和关系记忆 | `relationship_memory.py`、外部 Skill 的 `memory_store.py` | Go SQLite 存储，兼容原有协议与表结构 | 中到高：不仅是字段CRUD；还包括撤销、清理、范围配额、来源、暂停与撤回同意 |
| Codex 登录与生成 | `codex_bridge.py`、`quick_reply.py` | Go `os/exec`、JSONL、goroutine/context | 中：CLI 路径定位、模型能力、进程复用、独立临时会话、超时取消和结果校验 |
| 自定义 API 三协议 | `api_bridge.py` | Go `net/http` + 协议适配 | 中：保留认证、地址校验、分页、来源额外头和错误脱敏；密钥仅在内存中 |
| 草稿润色、历史与资讯话题 | `assistance.py`、`topic_news.py` | Go Schema/校验、HTTP/XML、短期缓存 | 低到中：来源与日期必须由宿主校验，不能把偶然提及存成偏好 |
| 安装包 | PyInstaller、Python 打包辅助脚本 | Go 构建、PowerShell组装 + 现有 Inno Setup | 中：DLL、模型、NOTICE、数据目录和旧版升级需要完整验证 |

## OCR 方案核验

### 首选：ONNX Runtime + Go

- [onnxruntime_go](https://github.com/yalue/onnxruntime_go) 支持 Windows，依赖 CGo 及匹配的 ONNX Runtime DLL。
- 已阅读 [gopaddleocr](https://github.com/multippt/gopaddleocr) 源码，参考版本 `d4b7cc8b442974de8020337bb58495bef1c4efe0`，仅放在 Git 忽略的 `upstream/` 中作分析，尚未引入生产依赖。
- 该参考实现要求 Go 1.25，默认识别模型/字典为 v5；本项目当前使用 PP-OCRv4。它提供模型配置入口，但不能据此认定已经兼容本项目；必须核对字符字典、输入输出、预处理和解码。
- 本机当前 Go 工具链为 1.26.4，但 `CGO_ENABLED=0`，没有找到 gcc/g++。需要准备 CGo 编译工具链并固定模型与 DLL 版本，之后才有条件进行可执行原型验证。
- 初步建议保留现有 v4 模型，仅替换宿主语言，先测一致性；若要升级 v5，应作为另一个可比较、可回退的实验。

### 备选

- [gosseract](https://github.com/otiai10/gosseract/blob/main/knowledge/windows-support.md)：Go 包装 Tesseract，Windows 构建涉及 CGo、原生库和字库。尚未做中文微信场景准确率对比，不建议直接替换当前引擎。
- [Windows.Media.Ocr](https://learn.microsoft.com/en-us/uwp/api/windows.media.ocr)：语言包和桌面应用包身份限制需要验证，不能假设当前未打包的 Wails 程序可以直接使用。
- 原生引擎仍需加载模型。移除 Python 可以减少解释器/子进程开销和安装体积，但目前不能量化 Go 迁移带来的速度收益。

## 这次识别等待的排查

已确认代码存在一个静止画面的重试缺口：

1. 当第一帧无法识别消息区时，捕获回调丢弃帧，没有向采集循环提供诊断/重试图像。
2. 当第一帧标题 OCR 失败时，采集循环请求下一帧；如果图像没变化，差分捕获不会再交帧，因此可能等到滚动/新消息才继续。
3. 第一帧文字全部被完整性过滤时，也可能没有下一次处理机会。

当前修复保留最近的非黑帧，并在首次标题/文字识别尚未完成时，每秒最多用该帧重试一次。重试仍遵守身份、单聊和半截消息保护，不编造聊天或发送时间；已成功识别正常文字后恢复按变化处理。

源码启动和构建输出同时统一为 `chat-junshi.exe`，不再优先启动另外的 `chat-junshi-fast.exe`。之前多个构建文件并存存在旧版本混用的可能，但没有证据证明这就是用户这次等待的原因。

本机一次隔离采集服务测试：预热约1.06秒，开启采集后约1.45秒收到8条消息。测试只报告阶段状态和数量，未提交实际聊天内容。真实 GUI 的自动测试由于 Windows 控制工具授权超时未完成，所以这些数据不能当成用户当前 GUI 长等待已被完全复现或解决的证据。

## 建议迁移顺序与验收门槛

1. **采集/OCR 技术原型**：精确定位同一窗口，以相同模型和固定虚构截图比较结果。记录窗口定位、首帧、检测、识别、标题和首批消息分别耗时；覆盖静止初帧失败、DPI、遮挡、最小化与窗口重启。原型不要读取微信数据库或自动发送。
2. **基础业务与协议**：移植聊天、时间、API、Codex、资讯与任务校验，保留现有前端 `Call` 动作和状态字段。把原 Python 测试的契约逐项迁到 Go，不为了快速编译而删除功能或测试。
3. **记忆兼容**：通过 Go SQLite 驱动读取同结构数据库，例如 [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) 提供不依赖 CGo 的实现。使用虚构数据库对照 `settings`、`memories`、`operations` 和同意/暂停/撤销/遗忘/删除语义。首次迁移先备份，故障可回退，不修改外部 Skill 的默认全局库。
4. **切换与安装包**：同一前端切换 Go/Python 后端作回归；Go版通过功能、准确率、耗时和旧数据读写检查之后，再去掉生产 Python 进程及 PyInstaller，保留必要的 OCR DLL、模型和许可证。

全量切换的完成标准：无 Python 运行进程与运行时要求；原有回复、润色、话题、记忆和导入导出功能齐全；旧数据无需用户重新录入；聊天/记忆仍按对象隔离；显式同意与撤销保持有效；OCR 质量不因缩短启动时间而明显下降。

## Git 与提交边界

本项目在 `chat-junshi/` 目录中以普通 `git clone` 下载，`--is-shallow-repository` 为 `false`，原仓库的两条历史提交均保留；外层空仓库没有用于本项目提交。参考 OCR 项目的浅克隆仅用于隔离分析，不影响本项目历史。

提交仅包含代码、测试和文档。聊天数据、关系数据库、截图、依赖/模型、API密钥、登录凭据及生成的基准结果不进入 Git。工作区中原有旧 Tkinter 文件删除属于另外的本地改动，未在本次提交中擅自恢复或归入迁移成果。
