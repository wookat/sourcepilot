<h1 align="center">贸灵 TradeMind</h1>

<p align="center">
  <strong>开源 AI 跨境电商运营平台</strong>
</p>

<p align="center">
  以运营者业务闭环为主线：采集选品 → AI 优化 → 商品草稿 → 货源 / SKU → 订单 → 采购 → 库存 → 发货 → 经营数据
</p>

<p align="center">
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white">
  <img alt="React" src="https://img.shields.io/badge/React-18+-61DAFB?logo=react&logoColor=111">
  <img alt="TypeScript" src="https://img.shields.io/badge/TypeScript-5+-3178C6?logo=typescript&logoColor=white">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white">
  <img alt="Self-hosted" src="https://img.shields.io/badge/Self--hosted-supported-2EA043">
</p>

<p align="center">
  简体中文 | <a href="README.en.md">English</a>
</p>

<p align="center">
  <a href="#快速开始">快速开始</a> ·
  <a href="#界面预览">界面预览</a> ·
  <a href="#业务闭环与核心能力">核心能力</a> ·
  <a href="#架构与技术栈">架构与技术栈</a> ·
  <a href="docs/README.md">文档中心</a>
</p>

<p align="center">
  <img src="docs/assets/img/readme-hero-zh.png" alt="TradeMind 产品预览" width="100%" />
</p>

TradeMind 是一个面向跨境卖家与开发团队的开源 AI 运营平台，围绕运营者每天真实发生的业务闭环构建：从 1688 / 拼多多等货源平台采集选品，用 AI 优化标题、描述和图片，整理成可刊登的商品草稿，绑定货源与 SKU，同步和处理平台订单，聚合生成采购单，管理库存与发货，最后回看经营数据。

项目当前聚焦两条主线：`AI 商品运营工具` 与 `多平台跨境 ERP MVP`。与传统重型 ERP 不同，TradeMind 不追求多仓、财务、WMS / OMS 的一次性全量覆盖，而是提供一个可私有化部署、可二次开发、可通过 Provider 扩展的平台底座。

## 业务闭环与核心能力

```text
采集选品 → AI 内容优化 → 商品草稿 → 货源 / SKU 绑定 → 刊登
    ↑                                                  ↓
经营数据 ← 发货履约 ← 库存协同 ← 采购协同 ← 订单同步与处理
```

### 采集与选品

- 商品采集：1688、拼多多、淘宝 / 天猫专用采集器与自定义规则采集，支持单条与批量提交。
- 采集运维：采集任务状态追踪、失败重试、采集监控（Worker / 任务 / 批次）、浏览器登录状态检测。
- AI 选品：选品任务与可上架清单，辅助从采集结果中筛选可运营商品。

### AI 内容与商品草稿

- AI 文案：标题优化、描述生成、Prompt 技能模板、结果对比、人工应用与撤销。
- AI 图片：抠图、翻译等图片任务，支持 remove.bg、OpenAI Image、ComfyUI 等 Provider，异步队列执行。
- 批量 AI：批量文案任务、批量图片任务与批次复核流程，配合商品运营工作台统一处理待办。
- 商品草稿：统一管理商品、SKU、图片、库存阈值与发布前检查（Readiness），支持运营进度追踪。

### 刊登与店铺

- 店铺授权：抖店 OAuth 闭环、敏感配置加密存储与连接测试。
- 商品刊登：多平台刊登中心、单商品与批量草稿创建、批量发布、草稿映射、发布任务、失败恢复与人工校正。

### 订单 → 采购 → 库存 → 发货

- 订单全生命周期：订单同步、手动创建与批量导入、规格（SKU）匹配、批量标记已付款 / 批量发货 / 批量导出发货清单。
- 异常工作台：聚合规格未匹配、扣库存失败、库存同步失败、采购受阻、利润为负等需人工处理的问题。
- 采购协同：按货源档案从销售订单聚合生成采购单，导出清单人工下单并回填 1688 订单号 / 运单号（人工下单过渡模式），支持批量提交 / 确认 / 标记付款 / 签收。
- 货源管理：供应商管理与商品货源档案，打通「订单 → 采购」的货源依据。
- 库存协同：库存中心、库存预警、扣减记录、库存流水、平台同步任务与批次。
- 经营报表：近 30 天订单数、付款数与销售额趋势，与工作台经营概览口径一致。

### 平台治理与工程化

- 权限矩阵：用户与权限管理、店铺归属隔离、导航与数据权限一致。
- 操作日志：登录、配置修改、任务操作、AI 应用等核心操作留痕。
- 客服协同：客服中心、会话列表、AI 建议回复与人工确认外发。
- Provider 架构：AI、存储、图片、平台、采集能力均通过 Provider 抽象扩展。
- 可靠性地基：关键写路径统一幂等，AI 结果应用 / 撤销保护，Webhook 快速 ACK，Worker 租约防止陈旧写回。
- 演示数据：内置 demo seed 脚本，一键造出覆盖全链路的演示数据集。

## 界面预览

以下截图来自 Docker Compose 全栈环境 + demo 演示数据。

<table>
  <tr>
    <td width="50%" align="center">
      <img src="docs/assets/img/r54-dashboard.png" alt="运营总览" width="100%" />
      <br />
      <sub><strong>运营总览</strong>：新手入门引导与今日运营概览</sub>
    </td>
    <td width="50%" align="center">
      <img src="docs/assets/img/r54-collect-hub.png" alt="采集中心" width="100%" />
      <br />
      <sub><strong>采集中心</strong>：多来源采集入口与登录风险提示</sub>
    </td>
  </tr>
  <tr>
    <td width="50%" align="center">
      <img src="docs/assets/img/r54-product-drafts.png" alt="商品草稿" width="100%" />
      <br />
      <sub><strong>商品草稿</strong>：草稿列表、运营进度与发布前检查</sub>
    </td>
    <td width="50%" align="center">
      <img src="docs/assets/img/r54-ai-workbench.png" alt="AI 商品运营工作台" width="100%" />
      <br />
      <sub><strong>AI 商品运营工作台</strong>：文案 / 图片复核与待办统一处理</sub>
    </td>
  </tr>
  <tr>
    <td width="50%" align="center">
      <img src="docs/assets/img/r54-orders.png" alt="订单管理" width="100%" />
      <br />
      <sub><strong>订单管理</strong>：订单同步、筛选与批量操作</sub>
    </td>
    <td width="50%" align="center">
      <img src="docs/assets/img/r54-exceptions.png" alt="订单异常工作台" width="100%" />
      <br />
      <sub><strong>订单异常工作台</strong>：规格未匹配等异常聚合处理</sub>
    </td>
  </tr>
  <tr>
    <td width="50%" align="center">
      <img src="docs/assets/img/r54-inventory.png" alt="库存中心" width="100%" />
      <br />
      <sub><strong>库存中心</strong>：本地库存、SKU 绑定与平台同步状态</sub>
    </td>
    <td width="50%" align="center">
      <img src="docs/assets/img/r54-reports.png" alt="经营报表" width="100%" />
      <br />
      <sub><strong>经营报表</strong>：近 30 天订单与销售额趋势</sub>
    </td>
  </tr>
</table>

## 架构与技术栈

```text
React + Ant Design Pro（admin）
        ↓
Go + Gin + GORM（backend API）
        ↓                ↘
PostgreSQL          Redis 队列
                         ↓
        Node.js + Playwright（collector）
```

| 层级 | 技术栈 |
| --- | --- |
| Backend | Go + Gin + GORM |
| Admin | React + TypeScript + Ant Design Pro |
| Collector | Node.js + TypeScript + Playwright |
| Data | PostgreSQL + Redis |
| Deploy | pnpm workspace + Docker Compose |
| Extension Points | AI / Storage / Image / Platform / Collector Providers |

## 快速开始

### Docker 一键启动（推荐）

```bash
cp .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

Windows PowerShell：

```powershell
Copy-Item .env.docker.example .env
docker compose -f docker-compose.full.yml up -d --build
```

默认访问地址：

| 服务 | 地址 |
| --- | --- |
| Admin | <http://127.0.0.1:8000> |
| Backend Health | <http://127.0.0.1:8080/health> |
| Collector Health | <http://127.0.0.1:3001/health> |

默认管理员账号由 `.env` 中的 `ADMIN_BOOTSTRAP_EMAIL` / `ADMIN_BOOTSTRAP_PASSWORD` 决定（示例值见 `.env.docker.example`，生产环境务必修改为强随机值）。

### 演示数据（可选）

服务启动后，可一键造出覆盖「采集 → 草稿 → 订单 → 库存 → 报表」全链路的演示数据：

```powershell
pnpm seed:demo-data          # 演示数据集
pnpm seed:demo-permissions   # 演示角色与权限
```

Go 版全链路演示种子（幂等，可一键清理）：

```bash
pnpm seed:demo:full          # 生成 DEMO- 前缀全链路演示数据
pnpm seed:demo:full:verify   # 校验演示数据
pnpm seed:demo:full:clean    # 仅清理 DEMO- 前缀数据
```

Linux / macOS 需先安装 [PowerShell 7](https://learn.microsoft.com/powershell/scripting/install/installing-powershell)，再执行 `bash scripts/seed-demo-data.sh`。详见 [docs/DEMO_SEEDING_GUIDE.md](docs/DEMO_SEEDING_GUIDE.md)。

### 本地开发

```bash
pnpm install
pnpm install:collector:browsers
pnpm dev
```

常用命令：

```bash
pnpm check:dev        # 开发环境自检
pnpm dev:infra        # 仅启动 PostgreSQL + Redis
pnpm dev:backend      # 仅启动后端
pnpm dev:admin        # 仅启动管理端
pnpm dev:collector    # 仅启动采集服务
pnpm build:admin
pnpm build:collector
```

更多说明：

- [本地开发](docs/development.md)
- [Docker 部署](docs/docker-deployment.md)
- [生产部署（域名 + HTTPS）](docs/production-deployment.md)
- [环境变量](docs/env.md)

## 文档导航

- [docs/README.md](docs/README.md)：完整文档入口（使用 / 部署 / 开发 / 测试 / 安全分类索引）。
- [docs/operations-manual.md](docs/operations-manual.md)：日常运营操作手册（选品 → 上架 → 采购 → 发货）。
- [docs/development.md](docs/development.md)：本地开发、调试与常用命令。
- [docs/docker-deployment.md](docs/docker-deployment.md)：Docker Compose 完整部署与运维说明。
- [docs/api.md](docs/api.md)：API 契约、统一返回与鉴权说明。
- [docs/provider.md](docs/provider.md)：Provider 扩展机制与安全约束。
- [docs/architecture.md](docs/architecture.md)：系统架构、分层与数据流说明。
- [docs/branching.md](docs/branching.md)：分支策略与 PR 规则。

## 贡献与社区

- 贡献代码或文档前，请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。
- 安全问题请参考 [SECURITY.md](SECURITY.md)。
- 如果你愿意补充更好的截图、示例数据或文档，也非常欢迎提交 PR。
- 赞助方式见 [docs/sponsor.md](docs/sponsor.md)。

## License

本项目基于 [Apache License 2.0](LICENSE) 开源。
