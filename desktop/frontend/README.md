# 前端

React 19 / HeroUI 3 / Vite 7 / Tailwind CSS 4。

需要 Node 20.19+ 或 22.12+。

```powershell
npm ci
npm run build
```

开发时运行 `npm run dev`，访问开发服务器的 `/?preview` 查看虚构数据。
预览模式不会读取真实聊天、关系记忆或调用分析服务；获取模型按钮返回演示列表。
真实桌面交互请使用 `wails dev` 或构建后的程序。

API Key 只留在运行内存中，不写入 localStorage；地址、来源和模型选择会保存到本地界面设置。
