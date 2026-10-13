# 聊天军师 · Chat Junshi

面向 Windows 微信单聊的本地回复工作台：**读屏与 OCR → 核对原文及身份 → 关系背景与记忆 → 候选回复和优缺点 → 自行复制发送**。

桌面和业务使用 Go / Wails 2，界面使用 React 19 / HeroUI 3。Go负责采集调度、OCR、关系记忆和模型接入，直接调用原生WGC及ONNX Runtime；生产版不启动Python或Web服务。

> 本项目仍处于开发阶段。仅采集窗口中识别到的部分对话，不保证完整历史或 OCR 准确性。请核对原文与身份后再分析。

## 功能

- 按微信进程与精确窗口 HWND 采集，避免按标题误抓辅助工具。
- 本地PP-OCRv4模型，采集暂停 / 恢复，原文和说话人纠错。
- 独立回复目标：预设选项和自定义输入。
- 四步建档：用户、对象、关系背景和明确的记忆同意。
- 双方性别可选填写，支持自定义和不愿透露；不根据姓名、头像或聊天推断性别。
- 本地关系记忆：查看、暂停、恢复、撤销、忘记对象和删除。
- 聊天与档案 JSON 导入 / 导出，导入后重新核对身份或确认保存方式。
- Codex 登录和自定义 API；来源预设、获取模型、下拉选择和手动输入模型 ID。
- Codex接入优先使用桌面客户端自带CLI及其模型目录；未安装桌面版时才回退到PATH/npm CLI。
- 三种自定义协议：OpenAI Chat Completions、Anthropic Messages、Gemini。
- 紫罗兰渐变、应用内毛玻璃、浅色 / 深色 / 跟随系统、窗口置顶。

## 安装版

本地构建产物为 `release/ChatJunshi-Setup-0.1.0-x64.exe`，按当前用户安装，不需要安装 Python、Go 或 Node。需要 WebView2 Runtime；安装器检测缺少运行时会提示安装后再继续。Codex 模式仍需安装并登录 Codex CLI，自定义 API 模式填写自己的密钥即可。

安装版用户数据位于 `%LOCALAPPDATA%\ChatJunshi\data`，卸载默认保留；升级不迁移源码版数据，可通过导入导出迁移。安装包尚未签名，系统可能提示未知发布者。

## 源码环境与安装

需要 Windows 10 1903+ / 11、WebView2、Go 1.25+（启用CGo）、MinGW、Wails 2、Rust GNU工具链，以及 Node 20.19+ 或 22.12+。Python仅用于可选的旧实现兼容对照测试。

源码运行可使用用户安装的 [goutoujunshi · 狗头军师 Skill](https://github.com/shengjidaguai-china/goutoujunshi)，通常位于 `%USERPROFILE%\.codex\skills\goutoujunshi`。安装包只包含固定公开版本的规则和参考；长期记忆由Go实现，兼容旧SQLite表，不再执行记忆脚本。

```powershell
rustup toolchain install stable-x86_64-pc-windows-gnu --profile minimal
# 传入现有v4模型目录和固定ONNX DLL；哈希见packaging/ocr-source.json
powershell -NoProfile -ExecutionPolicy Bypass -File packaging/prepare-runtime.ps1 -ModelDirectory D:\ocr\models -OnnxDll D:\ocr\onnxruntime.dll
powershell -NoProfile -ExecutionPolicy Bypass -File packaging/build-frontend.ps1
cd desktop
wails build -s -o chat-junshi.exe
```

也可在环境准备好后双击 `build-desktop.bat`。构建前保存草稿并退出同名程序，以免 Windows 文件锁影响构建。

双击项目根目录的 `run.bat` 启动新版桌面界面。

源码版运行需保留项目目录及`.go-cache/runtime`资产。现有工作区可从旧`.venv`提取固定模型和DLL作为构建输入，但运行时不依赖虚拟环境。安装版内置原生库、OCR及公开MIT狗头军师规则；构建流程见 [Windows 安装包](packaging/README.md)。

## 日常流程

源码启动统一使用 `desktop/build/bin/chat-junshi.exe`，`run.bat` 与 `build-desktop.bat` 使用同一个输出名，避免不同版本可执行文件混用。首次识别失败时会用最近的非黑帧定时重试，无需一直等滚动或新消息才重新处理。完整 Go 迁移的可行性、依赖和验收门槛见 [Go 迁移分析](docs/go-migration.md)。

程序启动后会在后台用空白图片初始化 OCR 模型，预热期间不定位或截取微信。显示“OCR 已准备好”后再开启采集，可避免把首次模型初始化等待放在采集按钮之后。暂停再恢复会复用同一进程的模型，但重新定位微信窗口；完全退出后模型释放，下次启动重新预热。

未识别时间的消息会按同一会话前后已有的明确时间或微信时间锚点推算范围：双侧有依据显示区间，单侧有依据显示之前/之后，并标记“推算，待核对”。前后时间矛盾时保持未知，不用采集时间补造发送时间，也不把推算时间用于精确回复间隔。旧记录、导出及模型输入均带推算信息；修改或删除时间依据后重新计算。在纠正弹窗明确“将时间设为未知”会关闭该条消息的自动推算。

OCR 会检查气泡是否被消息区上下边缘截断，并检查可见文字行是否存在明显漏识别；可疑气泡暂不保存，完整显示后再识别。多行正文按气泡背景边界合并，避免只靠文字行距拆开同一条消息。这些检查不能保证所有 OCR 结果都完整或准确，仍需核对原文。

每条消息的铅笔旁有单条删除按钮，确认后仅移除本工具中的这一条记录，并使相关建议和候选预览失效；微信原文、其他消息和关系记忆保留。删除后其他消息编号不变化，新消息也不复用已删除的编号。

回复工作台有“回复建议”“润色草稿”“找话题”三个入口。润色保留用户草稿的原意、事实、承诺及拒绝边界，提供自然、简短、温柔、轻松或表达边界等方向，原草稿保留便于修改。草稿和补充兴趣仅在当前运行中按会话隔离，不自动保存为关系记忆；关闭程序后不保留。

找话题参考最近80条已采集消息、当前对象资料、已启用记忆和本次补充兴趣，区分明确喜欢、只是提过、暂定推测与未知。话题卡提供开场白、适合理由、消息编号和后续方向；没有证据的通用开场标为“探索性话题”。新任务每次只调用一次所选模型，不额外调用排序模型，也不自动写入新偏好。

勾选“结合近期科技、游戏与文化资讯”后，程序获取 IT之家与机核的公开 RSS 标题、来源链接和发布日期，筛选近三天的内容并在内存缓存五分钟，不向资讯源发送聊天、草稿、兴趣或档案。模型只可引用实际取得的来源，相关链接由宿主校验；这些是近期资讯，不保证代表全网热度。来源不可用、缺少有效日期或没有适合的话题时回退到历史与兴趣，不编造热点。热点卡可点击链接查看原文。切换会话或修改输入后，旧结果失效。

采集会识别消息区居中的微信时间标签（时分、昨天、前天、星期、日期及上午/下午），保存原标签、按北京时间解析的锚点与精度。时间标签属于消息组，不冒充组内每条消息的精确发送时间；无标签的消息时间保持未知。消息旁显示时间，点击铅笔可核对并填写明确的北京时间，或设为未知。历史记录不会用采集时间补造发送时间。

每次生成回复都会读取本机时钟并转换成当前北京时间，提供当前时段与有效的明确消息间隔。模型据此考虑晚间、跨天、消息已经过去多久等情形；不根据单次延迟判断冷淡，也不把采集顺序等同于完整历史的真实响应时长。当前北京时间不代表对方所在时区。日期推断及 OCR 标签需要核对，聊天导入导出保留时间信息。

回复工作台默认使用“快速回复”：保留所选模型，直接提供精简的即时回复规则、当前对象档案及已启用记忆，减少临时读取 Skill 的等待。Codex 通道复用服务进程和模型列表，每轮仍创建独立的临时会话；模型支持时使用低思考强度。候选生成过程中可以预览，完整结果校验后才能复制。需要完整关系判断时选择“深入分析”，沿用原有 Skill 流程。

完成后可以展开“本次用时”查看连接准备、会话就绪、首条候选和总耗时；这些计时仅保留在本次运行的内存中。运行 `.venv\Scripts\python.exe tests\benchmark_reply.py` 可使用固定虚构聊天比较原流程与快速流程，结果保存到 `tests/reply-benchmark.json`，不使用微信原文或用户档案。单次测试受网络和服务负载影响，不代表固定回复时长。

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
| [goutoujunshi · 狗头军师 Skill](https://github.com/shengjidaguai-china/goutoujunshi) | 关系分析规则与参考；安装构建下载固定公开 MIT 版本，仅打包规则和参考，不打包 Python 脚本或个人档案 |
| [Wails](https://github.com/wailsapp/wails) | Go 桌面宿主及 WebView2 接入 |
| [React](https://github.com/facebook/react)、[HeroUI](https://github.com/heroui-inc/heroui)、[Tailwind CSS](https://github.com/tailwindlabs/tailwindcss)、[Vite](https://github.com/vitejs/vite) | 界面、交互组件、样式与前端构建 |
| [Lucide](https://github.com/lucide-icons/lucide) | 界面图标 |
| [RapidOCR](https://github.com/RapidAI/RapidOCR)、[ONNX Runtime](https://github.com/microsoft/onnxruntime) | 本地 OCR 识别与模型运行 |
| [PaddleOCR](https://github.com/PaddlePaddle/PaddleOCR) | PP-OCRv4 模型，来源及哈希见 `packaging/ocr-source.json` |
| [gopaddleocr](https://github.com/multippt/gopaddleocr)、[onnxruntime_go](https://github.com/yalue/onnxruntime_go) | Go OCR 识别、改编的检测代码与 ONNX 绑定；检测代码保留 Apache-2.0 许可 |
| [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) | Go 关系记忆及旧 SQLite 数据兼容 |
| [windows-capture](https://github.com/NiiightmareXD/windows-capture) | Windows 窗口采集绑定 |
| [Codex](https://github.com/openai/codex) | 可选的本机 App Server 分析通道 |

本项目是独立衍生项目，不表示由上述项目官方出品或背书。具体来源和完整上游声明见 [NOTICE](NOTICE) 与 [licenses](licenses/)。

## 许可证

本项目自身代码采用 [MIT License](LICENSE)，保留已有上游版权；`desktop/internal/ocr/detection.go` 改编部分采用 Apache-2.0，见 [NOTICE](NOTICE)。依赖、OCR 模型和外部 Skill 遵守各自许可。安装版保留 Go、Rust、前端、OCR 和 Skill 的许可与声明；构建及分发说明见 [packaging/README.md](packaging/README.md)。
