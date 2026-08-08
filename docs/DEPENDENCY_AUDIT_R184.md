# R184 admin 构建链依赖告警逐项评估（P2-4 收口）

- 输入：`pnpm audit --prod` 报 **16 项**（3 low / 8 moderate / 5 high），去重后为 **15 个 advisory**（pnpm 按受影响版本段计数，vite 存在同一 advisory 覆盖多段的重复计数），与 R183 审计第 5 节口径一致。
- 总结论：**全部位于 admin 构建 / lint 工具链（umi 4.6.83 的传递依赖）**，路径均以 `admin > @umijs/max@4.6.83 > …` 开头（@umijs/bundler-webpack、bundler-mako、bundler-vite、bundler-utoopack、@umijs/lint 五条链）；不进入任何生产运行时服务端路径（Go 后端与 collector 运行时不含这些包），vite/esbuild/launch-editor/@hono/node-server 仅本地 dev server 与构建期执行，image-size/nanoid/elliptic 为 bundler / lint 内部依赖。**无生产可利用面，本轮不升级。**
- 处置原则（与 R183 建议一致）：不跨 major 不动；同 major 有补丁的项若单独 override 需重验 umi 全链构建（四个 bundler 各带独立 vite/esbuild 副本），收益（dev-only 面）不抵稳定性风险，统一等 umi 升级窗口。

## 逐项清单（按 `pnpm audit --prod` 输出顺序）

| # | 严重级 | 包 | Advisory | 受影响 | 修复版本 | 评估 / 处置 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | high | vite（launch-editor 命令注入，Windows） | GHSA-c27g-q93r-2cwf | <=5.4.8 | >=5.4.9 | 本地 dev server + 仅 Windows；开发环境为 Linux。登记，随 umi 窗口 |
| 2 | high | vite（server.fs.deny Windows 备用路径绕过） | GHSA-fx2h-pf6j-xcff | <=6.4.2 | >=6.4.3 | 同上（仅 Windows）。登记，随 umi 窗口 |
| 3 | high | image-size（ICNS 解析 DoS） | GHSA-w3rx-r6r6-pgpr | <=2.0.2 | 无补丁 | 构建期图片探测，输入为仓库内资源，攻击面可控。无补丁可用，登记 |
| 4 | high | image-size（JXL/HEIF 解析 DoS） | GHSA-5p2g-fcmc-qvqq | <=2.0.2 | 无补丁 | 同上。无补丁可用，登记 |
| 5 | high | nanoid（自定义生成器死循环） | GHSA-2v37-7h3g-55p8 | <3.3.17 | >=3.3.17 | @umijs/lint > postcss 传递依赖，仅 lint 期，且项目不使用自定义生成器。同 major 有补丁，随 umi 窗口统一 |
| 6 | moderate | esbuild（dev server 允许任意站点发请求） | GHSA-67mh-4wv8-2f99 | <=0.24.2 | >=0.25.0 | 本地 dev server；0.24→0.25 为 esbuild 事实 breaking 版本。登记 |
| 7 | moderate | vite（server.fs.deny 反斜杠绕过，Windows） | GHSA-93m4-6634-74q7 | >=4.5.3 <5.0.0 | >=5.4.21 | 命中 override 钉住的 vite 4.5.14，修复需跨 major（4→5）。登记 |
| 8 | moderate | react-router（外部重定向） | GHSA-9jcx-v3wj-wh4m | <6.30.2 | >=6.30.2 | umi 内部路由传递依赖，admin 路由由 umi 配置生成、不拼接不可信 URL。同 major 有补丁，随 umi 窗口统一 |
| 9 | moderate | vite（Optimized Deps 路径穿越） | GHSA-4w7w-66w2-5vf9 | <=6.4.1 | >=6.4.2 | 本地 dev server。登记，随 umi 窗口 |
| 10 | moderate | vite（launch-editor NTLMv2 哈希泄露，UNC/Windows） | GHSA-v6wh-96g9-6wx3 | <=6.4.2 | >=6.4.3 | 仅 Windows dev。登记，随 umi 窗口 |
| 11 | moderate | @hono/node-server（路径穿越） | GHSA-frvp-7c67-39w9 | <2.0.5 | >=2.0.5 | @utoo/pack（umi utoopack bundler）内部 dev server；修复跨 major（1.x→2.x）。登记 |
| 12 | moderate | react-router（`<Link>` 反斜杠开放重定向） | GHSA-wrjc-x8rr-h8h6 | <7.18.0 | >=7.18.0 | 修复跨 major（6→7），umi 4 锁定 6.x。登记 |
| 13 | low | vite（同名前缀文件泄露） | GHSA-g4jq-h2w9-997c | <=5.4.19 | >=5.4.20 | 本地 dev server。登记，随 umi 窗口 |
| 14 | low | vite（server.fs 未作用于 HTML） | GHSA-jqfw-vq24-v9c3 | <=5.4.19 | >=5.4.20 | 同上。登记，随 umi 窗口 |
| 15 | low | elliptic（风险加密原语） | GHSA-848j-6mx2-7j84 | <=6.6.1 | 无补丁 | crypto-browserify（bundler Node polyfill 链）构建期依赖，产物不含服务端签名逻辑。无补丁可用，登记 |

## 复查触发条件

- umi / @umijs/max 出现可平滑升级的版本窗口时统一升级并重跑 `pnpm audit --prod`；
- 任一告警包出现生产运行时新引用（`pnpm why <pkg>` 出现非 umi/lint 链）时立即重评；
- 安全审计季度复跑（下次 R189 前后）例行复查计数与新增项。

## 附注

- 仓库 override 现以 `pnpm-workspace.yaml` 的 `overrides` 为准（pnpm ≥10 不再读取 package.json 的 `pnpm.overrides`，两处已保持同步）。
