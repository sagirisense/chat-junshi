# Windows 安装包构建

Go迁移版内置WGC原生DLL、PP-OCRv4模型、ONNX Runtime及固定公开MIT狗头军师规则，不捆绑Python运行时或脚本。目标系统为Windows 10 1903+ x64 / Windows 11，需WebView2。Codex CLI及登录信息不分发。

## 构建

准备Go 1.25+、MinGW/CGo、Wails 2、Rust GNU工具链、Node 20.19+或22.12+和Inno Setup 6。

```powershell
rustup toolchain install stable-x86_64-pc-windows-gnu --profile minimal
powershell -NoProfile -ExecutionPolicy Bypass -File packaging/fetch-skill.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File packaging/build.ps1 -ModelDirectory D:\ocr\models -OnnxDll D:\ocr\onnxruntime.dll -Iscc 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe'
```

模型及DLL必须匹配`ocr-source.json`的固定SHA256；`prepare-runtime.ps1`默认也能从旧工作区的`.venv`提取这些文件，不执行Python。可传`-VCRedistDirectory`指定VC143 x64可再分发DLL目录。字典在构建时从v4模型metadata提取；参考实现文件名`ppocr_keys_v5.txt`不表示模型升级。

`build-frontend.ps1`构建界面，`prepare-runtime.ps1`构建原生WGC，`assemble.ps1`只复制声明的运行资产、公开规则及许可证，生成每个文件的SHA256清单并拒绝Python与私有数据。旧 Python 打包脚本与 PyInstaller 依赖已移除；个人表情整理脚本仍保留，仅操作本机素材，见 [本地表情库](../docs/stickers.md)。

产物为`release/ChatJunshi-Setup-0.1.0-x64.exe`。安装包未签名。

## 数据与升级

程序位于`%LOCALAPPDATA%\Programs\ChatJunshi`，数据位于`%LOCALAPPDATA%\ChatJunshi\data`。升级移除旧安装的`service`目录，仅替换程序文件；卸载保留数据。

Go首次打开已有数据前，会在`data/migration-backup-go-v1`备份聊天JSON和一致SQLite快照，完成后写入`go-migration-v1.json`标记。备份失败时不打开业务服务。源码版与安装版数据仍独立，可通过导入导出迁移。

模型准确率、真实GUI采集和安装升级必须按`docs/go-migration.md`验收，编译成功不能替代这些检查。
