# 0001 - 用 SSE 替代 WebSocket 实现 AI 聊天流式输出

AI 助手的实时聊天采用 SSE（Server-Sent Events）而非 WebSocket，仅用于服务端向客户端推送 token 流。

选择 SSE 的原因：（1）AI 聊天是天然的单向推流场景——客户端发送一条消息，服务端持续返回 token，无需客户端频繁发消息，WebSocket 的双向能力用不上；（2）SSE 无需手动维护心跳和断线重连，实现远比 WebSocket 简单；（3）uni-app-x 的 `uni.connectSocket` API 在原生端（Android/iOS）的稳定性存疑，而 SSE 基于标准 HTTP，跨端兼容性更好。

其他非实时场景（通知、论坛更新）采用手动刷新/下拉刷新，不走任何长连接。
