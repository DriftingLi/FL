# 自动化测试操作指南

## 前置条件
- HBuilderX 已安装"自动化测试"插件
- 微信开发者工具已打开（测试微信小程序时）

## 测试文件结构
```
pages/
├── index/
│   ├── index.uvue          # 被测试页面
│   └── index.test.js       # 测试用例（与页面同级）
├── login/
│   ├── login.uvue
│   └── login.test.js       # 登录页测试
└── ...
```

## 在 HBuilderX 中运行测试

### 步骤 1：打开测试插件
1. 在 HBuilderX 顶部菜单栏，点击 **工具** → **自动化测试**
2. 或使用快捷键 `Ctrl+Shift+T`（Windows）/ `Cmd+Shift+T`（Mac）

### 步骤 2：选择测试平台
在测试插件界面中选择目标平台：
- **H5**：浏览器环境
- **微信小程序**：需要微信开发者工具
- **Android**：需要连接设备或模拟器

### 步骤 3：运行测试
1. 点击 **运行** 按钮
2. 等待测试执行完成
3. 查看测试结果

## 编写测试用例

### 基本语法
```javascript
describe('测试套件名称', () => {
  let page;

  beforeAll(async () => {
    // 启动页面
    page = await program.reLaunch('/pages/xxx/xxx');
    await page.waitFor(3000);
  });

  it('测试用例名称', async () => {
    // 获取元素
    const el = await page.$('.class-name');
    // 获取文本
    const text = await el.text();
    // 断言
    expect(text).toEqual('预期文本');
  });
});
```

### 常用 API

| API | 说明 | 示例 |
|-----|------|------|
| `program.reLaunch(url)` | 重启到指定页面 | `await program.reLaunch('/')` |
| `program.currentPage()` | 获取当前页面 | `await program.currentPage()` |
| `page.$(selector)` | 获取单个元素 | `await page.$('.title')` |
| `page.$$(selector)` | 获取多个元素 | `await page.$$('.list-item')` |
| `element.text()` | 获取元素文本 | `await el.text()` |
| `element.attribute(name)` | 获取元素属性 | `await el.attribute('class')` |
| `element.tap()` | 点击元素 | `await el.tap()` |
| `page.waitFor(ms)` | 等待指定时间 | `await page.waitFor(1000)` |

## 测试示例

### 登录页测试
```javascript
describe('登录页测试', () => {
  let page;

  beforeAll(async () => {
    page = await program.reLaunch('/pages/login/login');
    await page.waitFor(2000);
  });

  it('登录按钮存在', async () => {
    const btn = await page.$('.login-btn');
    expect(btn).toBeTruthy();
  });

  it('输入框可聚焦', async () => {
    const input = await page.$('.username-input');
    await input.tap();
    await page.waitFor(500);
    // 验证输入框获得焦点
  });
});
```

### 课程列表测试
```javascript
describe('课程列表测试', () => {
  let page;

  beforeAll(async () => {
    page = await program.reLaunch('/pages/courses/courses');
    await page.waitFor(3000);
  });

  it('课程列表不为空', async () => {
    const list = await page.$$('.course-item');
    expect(list.length).toBeGreaterThan(0);
  });

  it('点击课程跳转详情', async () => {
    const item = await page.$('.course-item');
    await item.tap();
    await page.waitFor(2000);
    const currentPage = await program.currentPage();
    expect(currentPage.path).toContain('course-detail');
  });
});
```

## 常见问题

### Q: 页面元素获取不到？
A: 检查选择器是否正确，微信小程序不支持父子选择器。

### Q: 分包页面测试报错？
A: 分包页面需要增加等待时间：
```javascript
await page.waitFor(7000);  // 分包页面等待时间
```

### Q: 测试超时？
A: 在 jest.config.js 中调整超时时间：
```javascript
testTimeout: 30000  // 增加到 30 秒
```

## 测试文件命名规范
- 文件名：`*.test.js`
- 放置位置：与被测试文件同级目录
- 示例：`pages/login/login.test.js` 测试 `pages/login/login.uvue`