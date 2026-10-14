# 构建资源

此目录保存 Wails 的图标、Windows 清单与安装脚本模板。
`bin/` 是构建输出目录，不提交 Git。

当前仅维护 Windows 运行环境。`darwin/` 来自 Wails 模板，不表示已经支持 macOS。
此目录中的安装器脚本是 Wails 模板；项目安装包使用根目录 `packaging/build.ps1` 与 Inno Setup 构建，见 [Windows 安装包构建](../../packaging/README.md)。Go 应用运行不依赖 Python 环境，安装包内置公开 Skill 规则和 OCR 资产。
