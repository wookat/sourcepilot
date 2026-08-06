# R134 依赖 advisories 登记（@umijs/max 构建链）

状态：**登记待决策，不擅动**（修复需跨 major 升级或 override 传递依赖，超出 P2 收口范围）。

`pnpm audit --prod`（2026-08-06，admin workspace）共 13 条：2 high / 8 moderate / 3 low，全部来自 `@umijs/max@4.6.83`（当前 4.x 最新线）构建链的传递依赖，不影响运行时后端与生产产物的服务端安全面，但属于构建/开发链暴露面：

| 严重度 | 包 | 问题 | 已修复版本 | 引入路径 |
| --- | --- | --- | --- | --- |
| high | vite | launch-editor 命令注入 | >=5.4.9 | @umijs/max > umi > @umijs/bundler-vite |
| high | vite | `server.fs.deny` Windows 绕过 | >=6.4.3 | 同上 |
| moderate | vite | fs.deny 反斜杠绕过 / Optimized Deps 路径穿越 / NTLMv2 泄露等 | >=5.4.21 / >=6.4.2 / >=6.4.3 | 同上 |
| moderate | esbuild | dev server 任意请求转发 | >=0.25.0 | @umijs/bundler-* |
| moderate | react-router | 外部重定向 / `<Link>` 反斜杠 open redirect | >=6.30.2 / >=7.18.0 | @umijs/max 路由层 |
| moderate | @hono/node-server | 路径穿越 | >=2.0.5 | umi dev server |
| low | vite / elliptic 等 | 静态文件边界 / GHSA-848j-6mx2-7j84 | 见 audit | node-libs-browser / crypto-browserify |

## 为什么不在本轮修

- 这些包由 umi 4.6.x 锁定：vite 4/5、esbuild 0.24、react-router 6.x 是 umi 内部选型，升到已修复版本需要 umi 发布新版本或本仓库使用 `pnpm.overrides` 强制跨 major 覆盖。
- 跨 major override（vite 5→6、react-router 6→7）有构建链行为变更风险，且 R134 指令明确「若需跨 major 则登记等待决策不擅动」。

## 缓解与现状

- 以上问题均只在 dev server / 构建环节可被利用；CI 与生产构建产物为静态资源 + Go 后端，不运行 vite/esbuild dev server。
- 本地开发 dev server 仅监听 127.0.0.1。

## 待决策项（下一轮）

1. 跟踪 `@umijs/max` 新版本是否升级 vite/esbuild/react-router；到位后直接升级 minor。
2. 若长期无修复，评估 `pnpm.overrides` 逐项 override 的构建回归成本（需全量门禁 + E2E 验证）。
