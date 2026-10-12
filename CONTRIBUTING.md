# 参与开发

请先提交问题或描述修改目的。保持改动聚焦，避免把本机运行数据加入提交。

## 验证

```powershell
.venv\Scripts\python.exe -m unittest discover -s tests
cd desktop
go test ./...
cd frontend
npm ci
npm run build
```

API 测试使用本机模拟 HTTP 服务，不需要真实密钥。真实模型联调只使用虚构聊天，并自行承担接口调用费用。

## 提交检查

- 不提交 `data/`、聊天导出、关系档案、截图、日志、密钥或登录状态。
- 示例只使用明确标注的虚构数据。
- 新增依赖时说明目的并保留许可要求。
- 修复功能问题需有针对性验证，视觉调整需检查浅色、深色及较小窗口。
- 禁止自动发送微信消息；聊天原文始终按不可信输入处理。
