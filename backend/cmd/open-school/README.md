# 公开网校前台（Go 模板 + htmx）

独立的公开课程前台：服务端直出 HTML，htmx 只负责「搜索 / 分类 / 翻页时替换结果片段」。
不写库、不转发任何凭据、不采集个人数据，只消费四条只读公开接口。

决策基线：#2747（技术栈改判 Go+htmx）、#2748（首批页面与内容边界）、#2749（首页/详情组织）、
#2751（学习入口交接）、#2752（路由归属与回退）、#2753（实施地图节点 D）。

## 本地启动

```powershell
# 后端预览 API 起在 18080（backend/docker-compose.yml 的端口映射）
cd backend
go run ./cmd/open-school
# → http://127.0.0.1:18090/
```

只绑回环，默认 `127.0.0.1:18090`。改端口用环境变量：

```powershell
$env:OPEN_SCHOOL_LISTEN = "127.0.0.1:18090"
$env:OPEN_SCHOOL_API = "http://127.0.0.1:18080"
$env:OPEN_SCHOOL_LEARN_BASE = "http://training.localhost:18091"
go run ./cmd/open-school
```

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `OPEN_SCHOOL_LISTEN` | `127.0.0.1:18090` | 监听地址。默认**只允许回环**（`127.0.0.1` / `::1` / `localhost`）；`0.0.0.0`、空主机、域名默认拒绝，见下一行。端口必须在 1–65535。 |
| `OPEN_SCHOOL_ALLOW_NON_LOOPBACK` | 关 | 显式允许绑定非回环地址，只认 `1` / `true`。容器里本进程与网关分属不同容器、网关按 Host 反代进来时**必须**打开（此时绑 `0.0.0.0`，绑回环则网关永远连不上，#2753 节点 E 实测）。置真后启动日志会 Warn 一条留痕。 |
| `OPEN_SCHOOL_API` | `http://127.0.0.1:18080` | 只读后端 API 基址。只接受 `http`/`https`，不允许带用户信息、查询串或片段；结尾斜杠会被规范化掉。 |
| `OPEN_SCHOOL_LEARN_BASE` | `http://training.localhost:18091` | 受信任学员站基址，用于「进入学习」的整页导航。约束同 `OPEN_SCHOOL_API`。 |

宿主机直跑（本文档主用法）不用管开关；容器里经网关接入时：

```powershell
$env:OPEN_SCHOOL_LISTEN = "0.0.0.0:8080"
$env:OPEN_SCHOOL_ALLOW_NON_LOOPBACK = "1"
$env:OPEN_SCHOOL_API = "http://backend:8080"
$env:OPEN_SCHOOL_LEARN_BASE = "http://training.localhost"
```

「默认安全」的价值在默认值而不是禁止：不置开关时非回环一律启动失败，关闭的那一步在部署清单里看得见。

配置全在 `config.go`，启动时一次校验完；校验失败进程直接退出并打印原因。

## 正式部署

Compose 的 `open_school` 服务从后端镜像启动独立进程，通过容器网络访问后端。切流前只暴露宿主机回环地址 `127.0.0.1:18090`，部署检查访问 `/question-banks` 验证进程和上游 API。

## 路由

| 路由 | 说明 |
| --- | --- |
| `GET /` | 公开目录（首页默认就是目录，#2749 A 稿） |
| `GET /courses` | 公开目录（筛选参数：`category_id` / `q` / `page` / `page_size`） |
| `GET /video`、`GET /illustrated` | 分别浏览视频课程、图文课程 |
| `GET /question-banks` | 按证件浏览题量与题型 |
| `GET /courses/{course_id}` | 课程详情（正式分享地址，不带浏览条件） |
| `GET /healthz` | 健康检查。**只报进程自身**，不探测上游 |
| `GET /assets/htmx.min.js` | 本地内嵌的 htmx 2.0.11 |
| `GET /assets/style.css` | 本应用唯一样式表 |
| `GET /assets/htmx-LICENSE.txt` | htmx 上游许可（Zero-Clause BSD） |

## 依赖的上游契约

统一信封 `{"code":200,"message":"success","data":...}`，错误信封 `data` 为 `null`。
契约实现在 `backend/internal/course`：

| 上游端点 | 用到的字段 |
| --- | --- |
| `GET {API}/api/public-school/categories` | `data.categories[].{id,name,course_count}` |
| `GET {API}/api/public-school/courses?category_id&q&page&page_size` | `data.{courses[].{course_id,name,description,cover_image,category{id,name},points_price},page,pages,total}` |
| `GET {API}/api/public-school/courses/{course_id}` | `data.{course_id,name,description,cover_image,category{id,name},points_price,chapter_count,chapters[].{chapter_id,title,order_num,content_type,duration}}` |
| `GET {API}/api/public-school/question-banks` | `data.question_banks[].{category_id,category_name,total,by_type}` |

口径约定（应用侧据此写，不要自行加字段）：

- 空分类项已由服务端隐藏，应用侧不再筛 `course_count`；
- 已发布的课程若所属一级培训分类已启用且映射到已启用的目标证件，即具备公开目录和详情的可见资格。清空专业方向或课程等级挂载后，符合这些条件的课程继续公开。
- 关键词匹配与「先筛后分页」都在服务端，应用侧**不**对整页结果再过滤；
- `points_price` 可能缺省（= 免费），应用侧把 `nil` 与 `0` 等同处理；
- 详情接口的「未发布 / 无有效分类 / 不存在」一律 HTTP 404；
- 详情接口 5xx 与网络错误 → 应用渲染「加载失败」错误态（状态码 **502**，可重试），**不**退化成空态或 404。

## 页面与局部更新

- 目录页提供分类、关键词、课程卡片与分页。
- 搜索 / 切分类 / 翻页都走 htmx：只替换 `#results` 片段，并用 `hx-push-url` 保留
  `category_id` / `q` / `page`。
- 片段判据（`handlers.go: isHTMXFragment`）：只有 `HX-Request: true` **且**
  `HX-History-Restore-Request` 不为 `true` 时才回片段。历史恢复（浏览器前进/后退）
  必须回整页——回片段会让整页被一段 `<section>` 顶掉。
- 详情页：权益区（免费 / N 积分解锁）+「进入学习」主操作 + 已发布章节元数据
  （序号 / 标题 / 类型 / 时长）。**无匿名试看**：不渲染章节正文、文件地址，也不渲染进度与权益态。
- 「进入学习」是普通整页导航到 `OPEN_SCHOOL_LEARN_BASE + /training/course/{course_id}`。
  基址只来自环境变量，不从 `Host` / `X-Forwarded-*` 推断，也不在这里探测角色或权益。
- 返回目录恢复原筛选：目录页把「当前筛选视图地址」写进卡片链接的 `back` 参数；
  服务端只接受本站 `/`、`/courses`、`/video` 或 `/illustrated` 的相对地址，绝对 URL、协议相对 URL、反斜杠、
  带片段、非目录路径一律退回 `/courses`（`handlers.go: safeBack`）。
- 分享元数据：详情页输出独立 `<title>`、`<meta name="description">`、
  `og:title/og:description/og:url`（有 `cover_image` 时另出 `og:image`）；
  `og:url` 用不带查询串的规范地址。404 与错误页带 `robots=noindex`。

## 静态资源

htmx 2.0.11 从 `backend/cmd/student-htmx-prototype/htmx.min.js` 复制（该原型目录保持原样，
未被修改），随 `//go:embed templates/*.html static` 编进二进制。上游许可见
`static/htmx-LICENSE.txt`（Zero-Clause BSD）。不引任何外网 CDN。

## 检查

```powershell
cd backend
go build ./cmd/open-school
go vet ./cmd/open-school
go test ./cmd/open-school
```

测试用 `httptest.Server` 假装上游，覆盖：整页/片段、查询串透传与分页钳制、详情权益与章节、
404 页、上游 500 与连接断开、分享元数据、`back` 白名单、学习链接、私有数据不泄漏。

## 边界（明确不做）

- 不写数据库、不落任何服务端状态；
- 不转发 Cookie / Authorization 到上游，不探测角色与权益；
- 不做匿名试看、不做积分兑换、不做学习进度——这些都属于学员站。
