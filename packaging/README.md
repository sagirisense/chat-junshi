# Windows 安装包构建

安装版内置 Python 运行时、RapidOCR 及 ONNX 模型、公开 MIT 狗头军师 Skill，默认按用户安装，无需管理员权限。
Codex CLI 和模型登录信息不随包分发；使用自定义 API 可不安装 Codex。
目标电脑需要 Windows 10 1903+ x64 / Windows 11 和 WebView2 Runtime。

## 构建

先按项目 README 准备 Python、前端、Go/Wails。另需 PyInstaller 和 Inno Setup 6 编译器：

```powershell
.venv\Scripts\python.exe -m pip install -r packaging/requirements-build.txt
.venv\Scripts\python.exe packaging/fetch-skill.py
cd desktop/frontend
npm ci
npm run build
cd ../..
powershell -NoProfile -ExecutionPolicy Bypass -File packaging/build.ps1 -Iscc 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe'
```

产物为 `release/ChatJunshi-Setup-0.1.0-x64.exe`。Skill 来源固定在 `skill-source.json`，只下载公开规则、参考、许可及记忆脚本，不复制用户安装的 Skill 或其个人数据。
`assemble.py` 只复制桌面构建输出、冻结服务和许可，生成包含每个文件 SHA256 的清单。

## 数据与卸载

程序文件位于 `%LOCALAPPDATA%\Programs\ChatJunshi`；用户数据位于 `%LOCALAPPDATA%\ChatJunshi\data`。
升级覆盖程序文件，卸载默认保留用户数据。请自行备份后通过应用或文件管理器清理数据。
源码版的旧数据不自动迁移，可导出导入，或退出程序后手动复制到新数据目录。

安装包目前没有代码签名。Windows 可能显示未知发布者提示。包中包含第三方许可证及明确的上游来源，分发前核对 `payload-manifest.json`。
