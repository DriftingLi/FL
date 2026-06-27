# Web SDK（AI 编程）
本文介绍如何安装并使用扣子编程 Web SDK，开发者可以参考本文档在自己开发的网站中快速添加一个 AI 智能体或工作流，为网站集成智能服务。
扣子编程提供两种不同的 Web SDK，分别适用于 AI 编程项目和低代码应用。本文档适用于 AI 编程项目，如需在网站中嵌入低代码应用，请参见[Web SDK（低代码）](https://docs.coze.cn/api/open/docs/developer_guides/ui_builder_web_sdk)。

## Web SDK 介绍
扣子编程 Web SDK 是专为 AI 编程场景设计的 Web 开发工具包，可帮助你将 AI 编程搭建的智能体与工作流快速集成到网页应用中。它提供 JavaScript、React 组件和 Iframe 多种嵌入方式，实现对话交互与业务自动化。
扣子编程 Web SDK 适用于需要在各类网页应用中快速集成扣子 AI 编程智能体和工作流的场景。

* **网页内嵌对话机器人**：在你的网站任意位置嵌入一个对话窗口，提供 7x24 小时的智能客服、产品导览、信息查询等服务。
* **触发式自动化工作流**：在用户完成特定操作（如填写表单、点击按钮）后，自动触发一个预设的工作流，完成数据处理、信息同步、发送通知等一系列后台任务。

**效果预览**
在网页应用中，开发者可以直接使用已部署的智能体与工作流。

* 智能体：可在网页内以对话界面形式呈现，用户能直接与智能体交互，获取智能问答、任务处理等 AI 能力。
   ![Image](https://p9-arcosite.byteimg.com/tos-cn-i-goo7wpa0wc/434a346c28d64b69879a96fe579158ec~tplv-goo7wpa0wc-image.image)
* 工作流：可在网页中直接触发和运行预设的自动化工作流，实现复杂业务流程的线上执行。
   ![Image](https://p9-arcosite.byteimg.com/tos-cn-i-goo7wpa0wc/0324ae55539a449aa03b66c8f81cfa00~tplv-goo7wpa0wc-image.image)

## 准备工作
接入 Web SDK 前，应完成以下准备工作：
| **项目** | **说明** |
| --- | --- |
| 部署并开启 Web SDK 渠道 | 已成功部署智能体或工作流，并开启了 Web SDK 渠道。详情请参见[部署智能体](https://docs.coze.cn/api/open/docs/guides/deploy_agent_as_api_service)、[部署工作流](https://docs.coze.cn/api/open/docs/guides/deploy_vibe_workflow)。 |
| 检查浏览器版本 | * Chrome：87.0 及以上  <br> * Edge：88.0 及以上 <br> * Safari：14.0 及以上 <br> * Firefox：78.0 及以上  |
| 获取 ProjectId | * **从 Web SDK 嵌入代码中获取**：项目部署后，平台会提供 Web SDK 的嵌入代码。 `ProjectId` 已被自动填充在代码中，可直接复制使用。 <br>    ![Image](https://p9-arcosite.byteimg.com/tos-cn-i-goo7wpa0wc/a49215b49cbd469c845572c420427e6c~tplv-goo7wpa0wc-image.image) <br> * **从浏览器地址栏获取**：进入项目详情页后，查看浏览器地址栏的 URL（格式示例：`https://code.coze.cn/p/:projectId/xxxx`），其中 `:projectId` 对应的字符串即为当前项目的 `ProjectId`。 <br>    ![Image](https://p9-arcosite.byteimg.com/tos-cn-i-goo7wpa0wc/dfe3a8eb62b541eb80706fe4f96074ea~tplv-goo7wpa0wc-image.image) |
| 获取访问令牌（Token） | * **支持的 Token 类型**：OAuth 访问令牌、个人访问令牌（PAT）和服务访问令牌（ SAT）。 <br> * **推荐的获取方式**： <br>    * **快速体验**：可使用个人访问令牌，详细说明可参考[添加个人访问令牌](https://docs.coze.cn/api/open/docs/developer_guides/pat)。 <br>    * **线上环境**：推荐使用 **OAuth JWT 授权（开发者）** 模式获取 Token，详情请参见[OAuth JWT 授权（开发者）](https://docs.coze.cn/api/open/docs/developer_guides/oauth_jwt)。 <br>    安全起见，建议使用 JWT 方式生成 Token 时，在 Payload 中添加`“session_context.connector_info.connector_id” : “2001”`。此参数控制 Token 的使用范围仅限于 Web SDK，以免 Token 泄露引发安全风险。 <br>  <br> * **权限要求** <br>    * `convertFileType` <br>    * `uploadFileToStorage` <br>    * `getMetadata(VibeProject)` <br>    * `run(VibeProject)` |
## 快速体验
以下是扣子编程 Web SDK 快速接入示例，采用 CDN 方式引入，帮助你在 Web 页面可视化体验智能体或工作流。

1. 新建 HTML 文件。
   在本地新建记事本，复制以下完整代码，粘贴后保存为 `coze-web-sdk-demo.html`，并保存至桌面。注意文件后缀必须为 `.html`，不可为 `.txt`。
   ```JavaScript
   <!doctype html>
   <html lang="en">
     <head>
       <meta charset="UTF-8" />
       <meta name="viewport" content="width=device-width, initial-scale=1.0" />
       <title>Coze Web SDK</title>
       <style>
         * { box-sizing: border-box;}
         html { margin: 0; height: 100%; }
         body { margin: 0; padding: 8px; height: 100%; }
       </style>
     </head>
     <body>
       <script src="https://lf-cdn.coze.cn/obj/unpkg/latest/coze/web-sdk/dist/js-umd/index.min.js"></script>
       <script>
         // 初始化 Web SDK。
         cozeWebSDK.init({
           // 替换为你的项目 ID，用于指定要加载的智能体或工作流。
           projectId: '761559609158873****',
           // 替换为 Token。作为初始化时的访问凭证，用于首次加载 Web SDK 时完成身份验证。
           refreshToken: () => Promise.resolve("czs_qrDe********"),
         });
       </script>
     </body>
   </html>
   ```

2. 替换鉴权信息，并保存文件。
   打开 `coze-web-sdk-demo.html` 文件，替换以下内容：
   * `你的Project ID`：替换为**准备工作**中获取的 Project ID。
   * `你的Token` ：替换为**准备工作**中获取的个人访问令牌。
3. 运行体验。
   双击打开 `coze-web-sdk-demo.html` 文件，等待 1-2 秒，即可在浏览器中直观体验智能体或工作流能力在网页中的呈现效果。

## 接入方式
扣子编程 Web SDK 支持以下三种接入方式。
| **接入方式** | **方案优势** | **适用场景** |
| --- | --- | --- |
| JavaScript | * 支持自定义触发方式、UI 样式与交互逻辑，可与产品深度融合。 <br> * 适配各类前端项目与静态页面。 | 适用于需要深度定制，且要求功能模块在视觉和交互上与产品无缝融合，以提供原生体验的场景。 |
| React 组件 | * 对 React 项目无缝集成，开发效率高。 <br> * 支持通过 Props 配置、生命周期管理实例，代码结构清晰易维护。 | 技术栈为 React 的项目首选。 |
| Iframe | * 仅需粘贴 HTML 代码，开发成本极低。 <br> * 沙箱安全隔离，无样式与脚本冲突风险。 | 快速上线、无需深度定制、优先保证稳定性的场景。 |
## 接入流程
你可以根据项目的定制需求和开发效率，选择合适的接入方式。
### JavaScript
采用 JavaScript 接入方式时，可参照以下操作步骤，同时结合对应的配置参考调整相关参数，确保接入效果符合需求。
#### 操作步骤

1. 安装或引入 Web SDK。
   你可以选择通过 npm 安装或直接在 HTML 中引用 CDN。
   * **方式一：使用 npm**
      ```Bash
      npm i @coze/web-sdk@latest
      ```

   * **方式二：引用 CDN**
      ```HTML
      <script src="https://lf-cdn.coze.cn/obj/unpkg/latest/coze/web-sdk/dist/js-umd/index.min.js"></script>
      ```

2. 初始化 Web SDK 实例。
   ```JavaScript
   // 如果使用 npm 包，需要先引入。
   import { cozeWebSDK } from '@coze/web-sdk/js'; 
    
   // 如果是引用 CDN，实例默认挂载在 window 对象上 (window.cozeWebSDK) 
    
   // 初始化 Web SDK。
   cozeWebSDK.init({ 
     // 替换为你的项目 ID，用于指定要加载的智能体或工作流。
     projectId: 'YOUR_PROJECT_ID', 
     // 替换为 Token。作为初始化时的访问凭证，用于首次加载 SDK 时完成身份验证。
     refreshToken: () => 'YOUR_TOKEN', 
     // 可选：指定挂载容器。
     // container: document.getElementById('sdk-container'), 
   });
   ```


#### 配置参考

   * **init(options: CozeWebSDKInitOptions): void（初始化 Web SDK 实例配置**）
      ```JavaScript
      type CozeWebSDKInitOptions {
          projectId: string;
          ...
      }
      ```

       初始化 Web SDK 实例可配置参数如下：
      | **参数** | **类型** | **是否必选** | **示例** | **说明** |
      | --- | --- | --- | --- | --- |
      | **projectId** | String | 是 | 761559609158873**** | 项目 ID，用于指定要加载的智能体或工作流。 |
      | **container** | Element \| String | 否 | js-cdn | 挂载的位置，默认挂载在`document.body`。 |
      | **theme** | String | 否 | light | 界面主题模式。取值如下： <br>  <br> * light（默认值）：明亮模式。 <br> * dark：暗黑模式。 |
      | **className** | String | 否 | coze-sdk-container | Web SDK 容器的 CSS 类名。 |
      | **style** | String | 否 | width: 100%; height: 600px; border: 1px solid #e5e7eb; border-radius: 8px; | 设置 Web SDK 容器的内联样式，例如宽高、边框等。 |
      | **refreshToken** | () => string \| Promise<string> | 否 | async () => { const res = await fetch('/api/get-coze-token'); const data = await res.json(); return data.access_token; } | 刷新访问令牌，支持返回字符串或 Promise<string>。当 `token` 未提供、已过期或鉴权失败时，SDK 会自动调用此方法获取新的访问令牌。 |
      | **token** | String | 否 | czs_hsk********** | 访问令牌，用于 Web SDK 鉴权。若传入此参数，Web SD 将优先使用该令牌进行身份验证，优先级高于 `refreshToken`。若无特殊需求，推荐直接使用 `refreshToken` 以实现自动续期。 |
      | **unauthorizedDescription** | String | 否 | 鉴权失败 | 未授权或鉴权失败时，界面展示的提示文案。 |
      | **onIframeReady** | () => void | 否 | \ | Web SDK 内部 iframe 加载并初始化完成。 |
      | **onIframeDestroy** | () => void | 否 | \ | 组件卸载或 iframe 被销毁。 |
      | **onProjectInfoLoaded** | () => void | 否 | \ | 成功获取到 AI 编程智能体或工作流项目的基础信息（如名称、图标等）。 |
      | **onTokenExpired** | () => void | 否 | \ | 当前 Token 已过期。 |
      | **onTokenInvalid** | () => void | 否 | \ | Token 校验失败（如格式错误、权限不足）。 |
      | **onNetworkError** | () => void | 否 | \ | Web SDK 内部请求发生网络故障。 |
      | **onNotify** | () => void | 否 | \ | 接收到 Web SDK 发送的系统通知或状态变更。 |
* **updateConfig(config: Config): void;  （更新 Web SDK 配置）**
   ```JavaScript
   interface Config {
      theme: string;
      ....
   }
   ```

   支持的配置项如下：
   | **参数** | **类型** | **是否必选** | **示例** | **说明** |
   | --- | --- | --- | --- | --- |
   | **theme** | String | 否 | light | 界面主题模式。取值如下： <br> light（默认值）：明亮模式。 <br> dark：暗黑模式。 |
   | **className** | String | 否 | coze-sdk-container | Web SDK 容器的 CSS 类名。 |
   | **style** | String | 否 | width: 100%; height: 600px; border: 1px solid #e5e7eb; border-radius: 8px; | 设置 Web SDK 容器的内联样式，例如宽高、边框等。 |
   | **unauthorizedDescription** | String | 否 | 鉴权失败 | 未授权或鉴权失败时，界面展示的提示文案。 |
* **updateToken(token: string): void;**
      ```JavaScript
      // 手动更新访问令牌。
      cozeWebSDK.updateToken('YOUR_NEW_TOKEN');
      ```

* **destory(): void;**
      ```JavaScript
      // 销毁 SDK 实例，移除 DOM 并解除所有事件监听。
      cozeWebSDK.destroy();
      ```


### React 组件
采用 React 组件 接入方式时，可参照以下操作步骤，同时结合对应的配置参考调整相关参数，确保接入效果符合需求。
#### 操作步骤

1. 安装 Web SDK 依赖包。
   在项目根目录下执行以下命令：
   ```Bash
   npm i @coze/web-sdk@latest
   ```

2. 引入并初始化组件。
   在你的 React 应用中，引入 `CozeWebSDK`组件并进行基础配置。最简单的渲染方式如下：
   ```JavaScript
   import { CozeWebSDK } from '@coze/web-sdk/react';
   
   const CozeChatPage = () => {
     return (
       <div style={{ height: '600px', width: '100%' }}>
         <CozeWebSDK
           // 替换为你的项目 ID，用于指定要加载的智能体或工作流。
           projectId="YOUR_PROJECT_ID" 
           refreshToken={async () => { 
             // 在 Token 过期时，自动获取 Token。
             const data = await fetch('https://YOUR_SERVICE/GET_TOKEN_API'); 
             const result = await data.json(); 
             return result.token; 
           }} 
         />
       </div>
     );
   };
   ```

3. 配置 Token。
   * **方式一：Token 过期时自动获取（推荐）**
      出于安全考虑，Token 通常具有较短的有效期。建议使用 `refreshToken` 属性，让 SDK 在 Token 过期时自动获取 Token，避免用户对话中断。
      ```JavaScript
      <CozeWebSDK
        // 替换为你的项目 ID，用于指定要加载的智能体或工作流。 
        projectId='YOUR_PROJECT_ID' 
        refreshToken={async () => { 
          // 在 Token 过期时，自动获取 Token。
          const data = await fetch('https://YOUR_SERVICE/GET_TOKEN_API'); 
          const result = await data.json(); 
          return result.token; 
        }} 
      />
      ```

      * 组件初始化完成后会立即调用一次`refreshToken`获取初始 Token。 
      * 当 Token 过期后，SDK 会自动调用`refreshToken`进行重试（最多等待 5000 毫秒）。 
      * 如果超时仍未获取到 Token，SDK 将上报`NETWORK_ERROR`事件。

   * **方式二：手动刷新 Token（不推荐）**
      通过 React 的 State 来管理 Token。出于安全考虑，不推荐将 Token 作为 State 频繁更新。如果使用此方式，建议你在后端服务为 Token 设置较短的过期时间。
      ```JavaScript
      // 填写准备工作中获取的 Token。
      const [token, setToken] = useState('YOUR_TOKEN'); 
       
      <CozeWebSDK 
        projectId='YOUR_PROJECT_ID' 
        token={token} 
      />
      ```


#### 配置参考
配置 CozeWebSDK 组件属性 ，以满足定制化需求。
| **参数** | **类型** | **是否必选** | **示例** | **说明** |
| --- | --- | --- | --- | --- |
| **projectId** | String | 是 | 761559609158873**** | 项目 ID，用于指定要加载的智能体或工作流。 |
| **theme** | String | 否 | light | 界面主题模式。取值如下： <br> light（默认值）：明亮模式。 <br> dark：暗黑模式。 |
| **className** | String | 否 | coze-sdk-container | Web SDK 容器的 CSS 类名。 |
| **style** | String | 否 | width: 100%; height: 600px; border: 1px solid #e5e7eb; border-radius: 8px; | 设置 Web SDK 容器的内联样式，例如宽高、边框等。 |
| **wrapClassName** | String | 否 | coze-sdk-wrapper | 外层包装元素的 CSS 类名，可用于全局样式控制。 |
| **wrapStyle** | String | 否 | margin: 20px auto; max-width: 1200px; | 外层包装元素的内联样式，例如居中、最大宽度、外边距等。 |
| **refreshToken** | () => string \| Promise<string> | 否 | async () => { const res = await fetch('/api/get-coze-token'); const data = await res.json(); return data.access_token; } | 刷新访问令牌，支持返回字符串或 Promise<string>。当 `token` 未提供、已过期或鉴权失败时，SDK 会自动调用此方法获取新的访问令牌。 |
| **token** | String | 否 | czs_hsk********** | 访问令牌，用于 Web SDK 鉴权。若传入此参数，Web SD 将优先使用该令牌进行身份验证，优先级高于 `refreshToken`。若无特殊需求，推荐直接使用 `refreshToken` 以实现自动续期。 |
| **unauthorizedDescription** | String | 否 | 鉴权失败 | 未授权或鉴权失败时，界面展示的提示文案。 |
| **onIframeReady** | () => void | 否 | \ | Web SDK 内部 iframe 加载并初始化完成。 |
| **onIframeDestroy** | () => void | 否 | \ | 组件卸载或 iframe 被销毁。 |
| **onProjectInfoLoaded** | () => void | 否 | \ | 成功获取到 AI 编程智能体或工作流项目的基础信息（如名称、图标等）。 |
| **onTokenExpired** | () => void | 否 | \ | 当前 Token 已过期。 |
| **onTokenInvalid** | () => void | 否 | \ | Token 校验失败（如格式错误、权限不足）。 |
| **onNetworkError** | () => void | 否 | \ | Web SDK 内部请求发生网络故障。 |
| **onNotify** | () => void | 否 | \ | 接收到 Web SDK 发送的系统通知或状态变更。 |
### Iframe
通过 Iframe 接入 Web SDK 的操作步骤如下。

1. 添加 Iframe 标签。
   在你的 HTML 页面中添加一个 `<iframe>` 标签。
2. 添加通信脚本。
   添加以下脚本来处理与 Iframe 的双向通信，包括发送初始化指令和响应 Token 更新请求。添加脚本时，需替换为实际的 `token`和`projectId`。
   ```HTML
   <script>
     const cozeWebSDK = document.getElementById("coze-web-sdk");  
     const COZE_WEB_SDK_ORIGIN = "https://sdk.coze.site";
     window.addEventListener("message", (event) => {
       // 只处理来自 SDK 的消息。
       if (event.origin !== COZE_WEB_SDK_ORIGIN) {
         return;
       }
       const data = event.data;
       // 监听到 IFRAME_READY 事件后，表示 WebSDK 已准备就绪。
       if (data.type === "IFRAME_READY") {
         // 初始化 WebSDK。
         cozeWebSDK.contentWindow.postMessage({
           type: "INIT",
           payload: {
             // 替换为你创建的 Token。作为初始化时的访问凭证，用于首次加载 WebSDK 时完成身份验证。
             token: 'YOUR_TOKEN', 
             // 替换为你的项目 ID，用于指定要加载的智能体或工作流。
             projectId: 'YOUR_PROJECT_ID'
           }
         }, COZE_WEB_SDK_ORIGIN);
       }
       // Token 过期通知。
       if (data.type === "TOKEN_EXPIRED") {
         // 更新 Token。
         cozeWebSDK.contentWindow.postMessage({
           type: "UPDATE_TOKEN",
           payload: {
             token: 'YOUR_NEW_TOKEN',
           }
         }, COZE_WEB_SDK_ORIGIN);
       }
     });
   </script>
   ```






