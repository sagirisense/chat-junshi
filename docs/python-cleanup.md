# Python 清理检查（2026-10-14）

删除前检查了 Git 跟踪文件、Python 导入关系、Go 子进程调用、启动批处理、PowerShell 构建链、测试及文档引用。生产路径为 `run.bat` → Go/Wails 可执行文件 → Go 业务服务、原生 WGC 和 ONNX Runtime。源码根目录通过 `desktop/wails.json` 定位，不依赖 Python 文件是否存在。

## 删除依据

| 删除范围 | 检查结果 |
| --- | --- |
| 根目录 Python 模块与 `app/*.py` | 旧 Tk 界面、JSONL 服务、采集/OCR、聊天、记忆、模型和资讯实现；引用来自旧 Python 应用与其测试，Go 生产代码不调用这些模块 |
| `run-legacy.bat`、根目录 `requirements.txt` | 仅用于旧 `main.py` 启动及 NumPy、RapidOCR、Python windows-capture 依赖，当前入口为 `run.bat` |
| `tests/*.py` | 测试/探针/性能比较依赖旧 Python 模块，不能验证 Go 运行路径；当前测试位于 `desktop` 的 Go 包中 |
| `packaging/assemble.py`、`packaging/fetch-skill.py`、`packaging/requirements-build.txt` | 旧 Python 服务安装包与 PyInstaller 路径；`packaging/build.ps1` 实际调用 PowerShell 同名替代脚本，不引用这些文件 |

删除仅针对上述跟踪文件，不递归清理本机环境、用户数据或构建缓存。历史源码可从 Git 历史恢复。旧 JSON/SQLite 数据兼容与 Go 首次迁移备份保留，上游版权、致谢及许可也保留。

## 保留依据

| 保留范围 | 用途 |
| --- | --- |
| `packaging/prepare-personal-stickers.py` | 从本机来源记录准备静态帧和总览图，需要 Pillow，详见 [本地表情库](stickers.md) |
| `packaging/refine-personal-sticker-tags.py` | 独立整理本机素材标签，不依赖旧应用模块 |
| `desktop/internal/ocr/similarity_test.go` | 可选调用 Python 标准库 `difflib` 验证 Go 相似度实现，不依赖删除的模块；通过 `JUNSHI_TEST_PYTHON` 显式启用 |
| `desktop/internal/memory/compatibility_test.go` | 可选验证 Go 与上游 SQLite 格式双向读写及撤销；启用前需自行准备 `packaging/vendor/goutoujunshi/scripts/memory_store.py` 和 `JUNSHI_TEST_PYTHON`，当前生产 Skill 下载脚本仅下载规则与参考 |
| 本机 `.venv` | `prepare-runtime.ps1` 默认从中提取固定模型/DLL，`assemble.ps1` 还读取依赖许可；不执行 Python，但直接删除会影响默认构建输入和许可收集 |
| `docs/capture-startup.md` | 明确标为历史排查记录，保留原因与证据，不作为当前操作说明 |

`.venv`、用户素材、聊天、关系档案、日志和缓存均保持本机状态并受 Git 忽略规则保护。保留可选测试与素材工具不意味着生产运行需要 Python。

## 验证方式

删除前后运行 `go test ./...` 与 `go vet ./...`，并检查删除文件的剩余引用。构建验证使用 `packaging/build-frontend.ps1` 和 Wails 构建。默认测试采用虚构数据；真实微信、模型供应商、旧安装升级以及可选原生/OCR/Python 对照测试须另行验收。

本次结果：移除 37 个跟踪文件，保留 2 个独立 Python 工具。删除前后 Go 全量测试及 vet、删除后的前端生产构建与 Wails 桌面编译均通过。额外显式运行旧库双向读写/撤销与 140 组相似度对照测试，两项通过。剩余旧文件名引用仅用于清理说明或明确标注的历史文档；没有保留的启动、生产或构建代码调用已删除文件。本次未重建安装包，也未执行真实微信 GUI 验收。
