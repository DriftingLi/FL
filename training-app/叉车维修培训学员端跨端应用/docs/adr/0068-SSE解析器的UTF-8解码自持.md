# SSE 解析器的 UTF-8 解码自持（不依赖 TextDecoder）

- 状态：已采纳（2026-11-12，#2457 实施同批写入；解析器本体见 #2388 / ADR-0060）

## 背景

`utils/sseIncrementalParser.uts` 原以类字段 `decoder : TextDecoder = new TextDecoder()` 解码完整前缀。微信小程序运行时（`mp-weixin`）没有 `TextDecoder` 全局，`createSseParser()` 构造即抛 `ReferenceError`，智能维修诊断在体验版上整条流式链路无回答（#2457 实测堆栈）。`api/aiAssistant.uts` 的 `bodyToText()` 同平台第二处：异常被 catch 吞掉后返回空串，整包降级路误报「回答未完整返回」。Node 测试缝里 `TextDecoder` 是全局，既有行为测试测不出。

## 决定

1. 删除 `TextDecoder` 字段；在解析器模块内新增 `export function decodeUtf8(bytes : Uint8Array) : string`，纯 JS 实现（`String.fromCharCode` + `Array<string>.join`），四字节码点输出 UTF-16 代理对。
2. 三端**单一路径**：`decodeChunk` 与 `bodyToText` 都只调 `decodeUtf8`，不按平台分支、不做运行期能力探测。
3. 尾部不完整序列的挂起规则不变（Encoding §8.1.1 `bytes needed`），只把完整前缀交给解码器；`finish()` 残留仍只置 `incomplete`、不产 U+FFFD。
4. 非法序列（孤立续字节 / 越界前导 / 超长编码 / 代理区码点）产 U+FFFD 且每次前进 1 字节，保证不抛、不吞后续合法字节；与 `TextDecoder` 的替换粒度不保证逐字节一致。
5. 行为守护：`utils/sseIncrementalParserMpWeixinBehavior.test.js` 在装载前删除 `globalThis.TextDecoder`，覆盖构造、跨块切分、整包降级，并与 Node `TextDecoder` 做差分。

## 后果

- 三端同一份解码实现、同一套替换口径，mp-weixin 不再需要 polyfill 或平台分支。
- 大响应体的解码从原生 `TextDecoder` 换成逐字节循环 + `join`，量级与既有逐字符 `pushText` 同阶；聊天分块场景无感。
- 既有行为测试（Node 有 `TextDecoder`）仍绿；新增套件补上「没有 `TextDecoder`」这一平台形状。

## 不采用的备选

- `#ifdef MP-WEIXIN` 两套实现：node 测试缝里两支同时执行，等于没分。
- 运行期 `typeof TextDecoder` 探测后 fail-open：UTS 编译期对未声明标识符不友好，且仍要维护第二条路径。
- 引入 `miniprogram-text-decoder` 之类的 polyfill：多一个包体积与依赖面，而解码器本身只有几十行。
