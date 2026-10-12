# 聊天军师 · Chat Junshi

面向 Windows 微信单聊的本地回复工作台：**读屏与 OCR → 核对原文及身份 → 关系背景与记忆 → 候选回复和优缺点 → 自行复制发送**。

桌面端使用 Go / Wails 2，界面使用 React 19 / HeroUI 3。Python 负责采集、OCR、关系记忆和模型接入。生产版通过本地标准输入输出通信，不运行 Web 服务。

> 本项目仍处于开发阶段。仅采集窗口中识别到的部分对话，不保证完整历史或 OCR 准确性。请核对原文与身份后再分析。

## 功能

- 按微信进程与精确窗口 HWND 采集，避免按标题误抓辅助工具。
- 本地 RapidOCR，采集暂停 / 恢复，原文和说话人纠错。
- 独立回复目标：预设选项和自定义输入。
- 四步建档：用户、对象、关系背景和明确的记忆同意。
- 本地关系记忆：查看、暂停、恢复、撤销、忘记对象和删除。
- 聊天与档案 JSON 导入 / 导出，导入后重新核对身份或确认保存方式。
- Codex 登录和自定义 API；来源预设、获取模型、下拉选择和手动输入模型 ID。
- 三种自定义协议：OpenAI Chat Completions、Anthropic Messages、Gemini。
- 紫罗兰渐变、应用内毛玻璃、浅色 / 深色 / 跟随系统、窗口置顶。

## 安装版

本地构建产物为 `release/ChatJunshi-Setup-0.1.0-x64.exe`，按当前用户安装，不需要安装 Python、Go 或 Node。需要 WebView2 Runtime；安装器检测缺少运行时会提示安装后再继续。Codex 模式仍需安装并登录 Codex CLI，自定义 API 模式填写自己的密钥即可。

安装版用户数据位于 `%LOCALAPPDATA%\ChatJunshi\data`，卸载默认保留；升级不迁移源码版数据，可通过导入导出迁移。安装包尚未签名，系统可能提示未知发布者。

## 源码环境与安装

需要 Windows 10 1903+ / 11、WebView2、Python 3.10–3.12、Go、Wails 2，以及 Node 20.19+ 或 22.12+。

另需由用户自行安装 [goutoujunshi · 狗头军师 Skill](https://github.com/shengjidaguai-china/goutoujunshi)，目录通常为 `%USERPROFILE%\.codex\skills\goutoujunshi`；本仓库不分发该 Skill 的正文、知识库或记忆脚本。两种模型接入均读取该 Skill 的规则；长期记忆复用其 `scripts/memory_store.py`。

```powershell
python -m venv .venv
.venv\Scripts\python.exe -m pip install -r requirements.txt
cd desktop\frontend
npm ci
cd ..
wails build -o chat-junshi.exe
```

也可在环境准备好后双击 `build-desktop.bat`。构建前保存草稿并退出同名程序，以免 Windows 文件锁影响构建。

双击项目根目录的 `run.bat` 启动。旧 Tkinter 界面保留为 `run-legacy.bat`，主要维护新版桌面界面。

源码版运行需保留完整项目目录和虚拟环境。安装版内置 Python、OCR 及公开 MIT 狗头军师 Skill；构建流程见 [Windows 安装包](packaging/README.md)。

## 日常流程

1. 打开微信单聊，保持窗口未最小化；点击顶部“开启采集”，也可导入本工具导出的聊天 JSON。
2. 首次遇到对象时按需建档。未知项留空，可先处理眼前回复。
3. 核对原文和说话人；勾选“已核对说话人，当前为单聊”。
4. 选择本轮回复目标，点击“生成回复建议”。
5. 查看事实、推测、未知和三条候选的优缺点，自行选择、复制和发送。

切换会话或纠错后旧建议作废。删除当前会话会删除其本地聊天文件并移出列表；关系记忆独立保留。

## 模型设置

顶部独立工具栏的“模型设置”提供两条路径：

| 接入方式 | 配置流程 |
| --- | --- |
| Codex 登录 | 自行安装并登录 Codex CLI → 获取模型 → 选择模型，或留空使用 CLI 列表默认项 |
| 自定义 API | 选择来源 → 填写 API Key → 获取模型 → 选择模型或手动输入 ID |

常见来源预设参考 JevChat，包括 DeepSeek、OpenRouter、OpenAI、Kimi、智谱、通义、硅基流动、OpenCode Go、Anthropic 和 Gemini。API 地址可修改。模型列表不保证聊天能力或调用权限；各提供商兼容性需实际验证。

来源、地址及模型选择保存到本工具的本地界面设置。API Key 仅本次运行有效，退出后需重新填写，切换来源会清空密钥。获取模型列表不提交聊天。

Codex 模式原生调用 Skill；自定义 API 模式读取本地 Skill 及回复参考作为系统指令，不运行 Codex 工具。两条路径使用相同的结果校验与宿主记忆管理流程。

## 数据、记忆与隐私

**聊天文字**保存在本机 `data/*.json`，包含采集到的敏感原文；时间分为屏幕可知的消息时间和采集观察时间，不用采集时间推断回复间隔。

**精简关系记忆**默认关闭，明确同意后保存在项目 `data/relationship-memory/`，不修改外部 Skill 的默认全局记忆库。仅召回当前对象；用户档案为共享信息，对象与关系信息按会话标题隔离。相同昵称仍可能冲突，需自行核对身份。

- “仅用于本次”不把新建档字段写入长期记忆。
- 明确填写的事实、评分和 MBTI 按用户提供的信息保存，未知项不代填。
- 分析可提出最多三条有消息编号支持的关键事件或暂定推测；由宿主校验保存，不存整份聊天或完整分析。
- 暂停记忆时不召回或写入。可撤销最近一次字段更新、忘记对象、撤回同意或删除全部关系记忆。
- 只有点击分析才提交最近 30 条消息、当前档案和已启用的当前对象记忆至所选模型服务。
- 不读取微信数据库、不 hook、不注入、不自动发送。截图处理在本机内存中，OCR 本地运行。
- 对话导入追加到同名会话；重复导入会产生重复消息。档案导入先填入编辑表单，确认后保存。

不要将聊天原文、关系档案、截图、密钥或登录凭据提交到公开仓库或 Issue。详见 [安全与隐私](SECURITY.md)。

## 开发与验证

```powershell
.venv\Scripts\python.exe -m unittest discover -s tests
cd desktop
go test ./...
cd frontend
npm run build
```

API 测试使用本机模拟服务和虚构数据，不需要真实密钥。前端可用 `npm run dev` 和 `/?preview` 查看虚构数据预览；桌面开发使用 `wails dev`。

测试覆盖身份确认、会话隔离、纠错、导入导出、记忆同意与隔离、旧建议作废、API 三协议解析、模型列表，以及无终端 PATH 时定位 Codex。构建成功不代表每个微信版本、DPI、模型或提供商都已通过真实联调。

欢迎参考 [参与开发](CONTRIBUTING.md)。

## 引用、参考与致谢

感谢以下开源项目和维护者：

| 项目 | 本项目使用或参考的部分 |
| --- | --- |
| [jev-chat-windows](https://github.com/jev-chat/jev-chat-windows) | 窗口采集、聊天应用识别、本地 OCR、模型来源预设与模型选择流程；参考版本 `c6d3a10474ce1ff3b7f8f6402b3b5e0e58fad5f1` |
| [jev-chat-jarvis](https://github.com/jev-chat/jev-chat-jarvis) | JevChat 上游项目背景；本项目未集成其 Jev 判断内核 |
| [goutoujunshi · 狗头军师 Skill](https://github.com/shengjidaguai-china/goutoujunshi) | 用户另行安装的关系分析规则、实战参考和受控记忆脚本；本仓库不分发其内容 |
| [Wails](https://github.com/wailsapp/wails) | Go 桌面宿主及 WebView2 接入 |
| [React](https://github.com/facebook/react)、[HeroUI](https://github.com/heroui-inc/heroui)、[Tailwind CSS](https://github.com/tailwindlabs/tailwindcss)、[Vite](https://github.com/vitejs/vite) | 界面、交互组件、样式与前端构建 |
| [Lucide](https://github.com/lucide-icons/lucide) | 界面图标 |
| [RapidOCR](https://github.com/RapidAI/RapidOCR)、[ONNX Runtime](https://github.com/microsoft/onnxruntime) | 本地 OCR 识别与模型运行 |
| [windows-capture](https://github.com/NiiightmareXD/windows-capture) | Windows 窗口采集绑定 |
| [Codex](https://github.com/openai/codex) | 可选的本机 App Server 分析通道 |

本项目是独立衍生项目，不表示由上述项目官方出品或背书。具体来源和完整上游声明见 [NOTICE](NOTICE) 与 [licenses](licenses/)。

## 许可证

本仓库代码采用 [MIT License](LICENSE)，保留已有上游版权。依赖、OCR 模型和外部 Skill 的许可分别以其实际发行版为准。分发包含依赖的安装包前应检查相应许可与 NOTICE；安装版保留所捆绑依赖和 Skill 的许可文本，构建及分发说明见 packaging/README.md。
