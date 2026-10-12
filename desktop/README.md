# 桌面宿主

Go / Wails 2 负责窗口、本地保存与 JSON 文件对话框，以及 Python 服务进程的生命周期。
前端使用 React 19、HeroUI 3 和 Vite。

从项目根目录执行 `build-desktop.bat` 构建，`run.bat` 启动。
完整的环境要求、运行依赖及隐私边界见 [项目说明](../README.md)。

开发环境可在此目录执行 `wails dev`。开发服务器只用于本地调试，生产版使用标准输入输出与 Python 通信。
