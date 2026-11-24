# GuliangIAM

这是新一代前端管理台的起步工程，用于替代旧的 CRA 管理台。

## 技术栈

- Vite 8
- React 19.2
- TypeScript 7.0+
- React Router 7
- Ant Design 6
- TanStack Query 5
- Zustand
- Zod

## 启动方式

1. 进入当前目录
2. 执行 pnpm install
3. 执行 pnpm dev

默认开发端口为 7101，接口代理到 <http://localhost:8000。>

## 当前已落地内容

- 管理台应用壳层
- 基于组织 navItems 的兼容菜单映射器
- 统一 API client
- Dashboard、Organizations、Users、Applications、Providers、Roles 六个资源页面骨架
- 会话加载与 Query 缓存基础设施

## 运行时说明

- Face ID 相关浏览器能力不再通过 `pnpm` 依赖参与 Vite 打包
- `web/public/vendor/face-api.min.js` 作为本地静态脚本随仓库维护
- 点击 Face ID 登录时，前端会并行预热识别弹窗模块与该静态脚本，以降低首次打开等待

## 代码质量

- 使用 oxlint 作为前端主 lint 工具
- 使用 oxfmt 作为前端主格式化工具
- 旧 web 目录仍保持渐进迁移，不在本轮直接替换全部旧工具链
