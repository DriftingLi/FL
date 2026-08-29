# ADR-0018: 论坛点赞/举报幂等设计

- 状态：已接受
- 领域：论坛模块 —— 点赞与举报操作

## 背景

论坛模块需要支持点赞（like）和举报（report）两种用户交互。这两种操作在社交产品中存在天然的重复提交场景：

- **点赞**：用户快速双击、网络延迟导致重复请求、页面刷新后重新操作
- **举报**：用户不确定是否提交成功而重复点击

传统的 "先查后写" 模式在并发场景下存在竞态条件，需要更可靠的幂等机制。

## 决策

### 点赞/取消点赞

**点赞**采用 INSERT ... ON CONFLICT DO NOTHING + 计数器累加：

- 点赞：INSERT INTO forum_likes (topic_id, user_id) VALUES (?, ?) ON CONFLICT (topic_id, user_id) DO NOTHING; 若 affected rows > 0 则 like_count += 1
- 取消点赞：DELETE FROM forum_likes WHERE topic_id = ? AND user_id = ?; 若 affected rows > 0 则 like_count -= 1（下限 0）

**幂等语义**：
- 重复点赞：返回 200 + 当前 like_count（不报错）
- 重复取消：返回 200 + 当前 like_count（不报错）
- 前端根据返回的 liked 状态更新 UI，不依赖请求成功/失败判断

**用户可给自己的帖子点赞**（确认）。

**计数器一致性**：forum_topics.like_count 为派生字段，由 INSERT/DELETE 的 affected rows 驱动增减。并发场景下计数器可能有微小偏差，但社交场景可接受。

### 举报

举报采用 UNIQUE(target_type, target_id, user_id) 约束 + ON CONFLICT DO NOTHING：

- INSERT INTO forum_reports (target_type, target_id, user_id, reason) VALUES (?, ?, ?, ?) ON CONFLICT (target_type, target_id, user_id) DO NOTHING

**幂等语义**：
- 重复举报：返回 200 + 提示文案「您已举报过该内容」
- report_count 计数器同样由 affected rows 驱动

**限制**：用户不能举报自己的帖子/回复（确认）。

**reason**：自由文本，1-500 字符（确认）。

**举报处理**：本期仅记录到 forum_reports 表，管理端后台查看；不通知管理员，不通知被举报用户（确认）。

## 与既有决策的关系

- ADR-0005（响应信封统一）：所有幂等操作返回统一信封 code: 200
- ADR-0009（handler 层站立模式）：点赞/举报 handler 遵循 Endpoint 模式

## 后果

**正面**：
- 幂等操作天然支持重试、网络抖动、用户重复点击
- 前端无需复杂的 loading 状态管理（乐观更新 + 幂等后端）
- 数据库约束保证数据一致性，无需应用层锁

**负面**：
- like_count 计数器在极端并发下可能有微小偏差（可接受）
- report_count 同理

## 相关

- backend/internal/service/forum_service.go
- backend/internal/model/models.go
- backend/internal/api/forum.go
- docs/design/forum-api-design.md