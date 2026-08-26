# 第三方开源组件声明（THIRD PARTY NOTICES）

本文件记录项目直接依赖中需保留版权声明的开源组件，满足其许可证义务。

## markstream-vue

- **Package**: `markstream-vue@2.0.4`
- **License**: MIT
- **Copyright**: Copyright (c) 2022 Simon He
- **Source**: https://github.com/Simon-He95/markstream-vue
- **声明原文**（`node_modules/markstream-vue/license`）：

```
MIT License

Copyright (c) 2022 Simon He

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- **使用范围**：仅 `frontend/src/pages/ai-assistant/AIAssistantPage.vue`（AI助手模块），`mode="chat"` 流式渲染，`htmlPolicy="escape"`，一期未启用 `stream-diffs`/`mermaid`/`katex` 等可选 peer。
- **依赖链**：`markstream-vue` → `markstream-core@2.0.4` (MIT), `stream-markdown-parser@1.2.11` (MIT) — 均为同作者 MIT，随主包一并满足。

> 后续若启用 `stream-diffs`（MIT）、`katex`（MIT）、`mermaid`（MIT）等 peer，将在本文件增量补录。
