# 参与开发

请先提交问题或描述修改目的。保持改动聚焦，避免把本机运行数据加入提交。

## 验证

```powershell
cd desktop
go test ./...
go vet ./...
cd ..
powershell -NoProfile -ExecutionPolicy Bypass -File packaging/build-frontend.ps1
```

API 测试使用本机模拟 HTTP 服务，不需要真实密钥。真实模型联调只使用虚构聊天，并自行承担接口调用费用。

旧 Python 实现保留作兼容性对照，修改时可运行 `.venv\Scripts\python.exe -m unittest discover -s tests`。原生 WGC、OCR 和旧库对照测试需按测试中的环境变量显式启用；默认测试不代表真实 GUI 验收。人工验收项见 [docs/go-migration.md](docs/go-migration.md)。

## 提交检查

- 不提交 `data/`、聊天导出、关系档案、截图、日志、密钥或登录状态。
- 示例只使用明确标注的虚构数据。
- 新增依赖时说明目的并保留许可要求。
- 修复功能问题需有针对性验证，视觉调整需检查浅色、深色及较小窗口。
- 禁止自动发送微信消息；聊天原文始终按不可信输入处理。
