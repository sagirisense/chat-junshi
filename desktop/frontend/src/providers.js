// Provider presets adapted from MIT jev-chat-windows/core/providers.py.
export const providers = [
  {
    "id": "deepseek",
    "name": "DeepSeek 官网",
    "protocol": "openai",
    "base": "https://api.deepseek.com"
  },
  {
    "id": "openrouter",
    "name": "OpenRouter",
    "protocol": "openai",
    "base": "https://openrouter.ai/api/v1"
  },
  {
    "id": "openai",
    "name": "OpenAI",
    "protocol": "openai",
    "base": "https://api.openai.com/v1"
  },
  {
    "id": "moonshot",
    "name": "Moonshot (Kimi)",
    "protocol": "openai",
    "base": "https://api.moonshot.cn/v1"
  },
  {
    "id": "zhipu",
    "name": "智谱 GLM",
    "protocol": "openai",
    "base": "https://open.bigmodel.cn/api/paas/v4"
  },
  {
    "id": "dashscope",
    "name": "通义千问",
    "protocol": "openai",
    "base": "https://dashscope.aliyuncs.com/compatible-mode/v1"
  },
  {
    "id": "siliconflow",
    "name": "硅基流动",
    "protocol": "openai",
    "base": "https://api.siliconflow.cn/v1"
  },
  {
    "id": "opencode",
    "name": "OpenCode Go",
    "protocol": "openai",
    "base": "https://opencode.ai/zen/go/v1"
  },
  {
    "id": "anthropic",
    "name": "Anthropic",
    "protocol": "anthropic",
    "base": "https://api.anthropic.com"
  },
  {
    "id": "gemini",
    "name": "Google Gemini",
    "protocol": "gemini",
    "base": "https://generativelanguage.googleapis.com/v1beta"
  },
  {
    "id": "custom_openai",
    "name": "自定义 · OpenAI 兼容",
    "protocol": "openai",
    "base": ""
  },
  {
    "id": "custom_anthropic",
    "name": "自定义 · Anthropic 兼容",
    "protocol": "anthropic",
    "base": ""
  }
]
