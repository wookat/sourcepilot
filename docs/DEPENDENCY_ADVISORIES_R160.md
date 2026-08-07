# 前端构建链依赖告警登记（R160）

`pnpm audit --prod` 共 13 条告警（2 high / 8 moderate / 3 low），全部来自 `@umijs/max@4.6.83` 构建链（dev server / bundler），不进入生产构建产物（生产为静态资源 + Go 后端）。本轮逐项评估结果：**0 条可在不跨 major 的前提下净收敛**，全部登记待决策，与 R134 结论一致（见 `docs/DEPENDENCY_ADVISORIES_R134.md`）。

## 逐项处置矩阵

| # | 级别 | 包 | 问题 | 修复版本 | 当前锁定 | 处置 |
| - | - | - | - | - | - | - |
| 1 | high | vite | launch-editor 命令注入 | >=5.4.9 | 4.x（umi bundler-vite） | 跨 major，登记 |
| 2 | high | vite | server.fs.deny Windows 备用路径绕过 | >=6.4.3 | 4.x | 跨 major，登记 |
| 3 | moderate | esbuild | dev server 任意请求转发 | >=0.25.0 | 0.18/0.21/0.24（umi bundler-*） | umi 锁定（0.x 语义即破坏性），登记 |
| 4 | moderate | vite | server.fs.deny 反斜杠绕过 | >=5.4.21 | 4.x | 跨 major，登记 |
| 5 | moderate | react-router | 外部重定向 | >=6.30.2 | 6.3.0（umi 路由层） | 本轮实测 minor 覆盖，见下，登记 |
| 6 | moderate | vite | Optimized Deps 路径穿越 | >=6.4.2 | 4.x | 跨 major，登记 |
| 7 | moderate | vite | launch-editor UNC 路径 NTLMv2 hash 泄露 | >=6.4.3 | 4.x | 跨 major，登记 |
| 8 | moderate | @hono/node-server | 路径穿越 | >=2.0.5 | 1.19.17（umi dev server） | 跨 major，登记 |
| 9 | moderate | react-router | `<Link>` 反斜杠 open redirect | >=7.18.0 | 6.3.0 | 跨 major（v7），登记 |
| 10 | low | vite | 静态文件同名前缀越界 | >=5.4.20 | 4.x | 跨 major，登记 |
| 11 | low | vite | HTML 文件不应用 server.fs | >=5.4.20 | 4.x | 跨 major，登记 |
| 12 | low | elliptic | 风险加密原语（GHSA-848j-6mx2-7j84） | 无修复版本 | 6.6.1（node-libs-browser 传递） | 无修复版本可用，登记 |
| 13 | low | vite（同链） | 见 audit | 见 audit | 4.x | 跨 major，登记 |

> 精确清单以 `pnpm audit --prod` 实时输出为准；证据快照存留于仓库外（R160 验证记录）。

## react-router minor 覆盖实测（本轮新增证据）

按「可补丁级修的修」口径实测 `pnpm-workspace.yaml` overrides 强制 `react-router@6 → 6.30.2 / 6.30.4`（同 major minor 升级，`build:admin` 可通过）：

- 升到 6.30.2 后 audit 反而 13 → 17 条：6.30.x 命中新增的 `>=6.4.0 <7.18.0` 区间告警，且引入 `@remix-run/router` 传递告警；
- 升到 6.30.4 后仍 14 条，其中 `react-router-dom >=6.30.2 <=6.30.4` 存在**无修复版本**的告警（Patched `<0.0.0`）；
- 结论：minor 覆盖不能净收敛、反而扩大告警面，已回退。react-router 线唯一净收敛路径是 v7（跨 major），继续登记待决策。

## 风险边界与缓解（沿 R134）

- 全部告警仅在本地 dev server / 构建环节可被利用；CI 与生产不运行 vite/esbuild dev server。
- 本地开发 vite dev server 仅绑定 localhost。
- 跟踪 `@umijs/max` 新版本是否升级 vite/esbuild/react-router；到位后以 minor 升级收敛。
