# 桌面宿主

Go / Wails 2 负责窗口、采集与OCR调度、聊天/记忆存储、模型接入和JSON文件对话框。生产入口不再启动Python服务。
前端使用 React 19、HeroUI 3 和 Vite。

从项目根目录执行 `build-desktop.bat` 构建，`run.bat` 启动。
完整的环境要求、运行依赖及隐私边界见 [项目说明](../README.md)。

先运行 `packaging/prepare-runtime.ps1` 准备 `.go-cache/runtime` 中的固定模型及原生库；开发环境可在此目录执行 `wails dev`。开发服务器只用于本地调试，生产版由Go直接调用业务服务。
