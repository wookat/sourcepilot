# DELIVERY_B — AI 比价选品引擎（线B / 后端工程师B）

分支：`feat/selection-engine`（基于 `dev`，仅本地提交，未推远端 / 未建 PR）

## 一、改动清单

### 后端（Go）

新模块 `backend/internal/modules/selection/`：

| 文件 | 说明 |
| --- | --- |
| `model.go` | 4 张表：`selection_tasks` / `selection_candidates` / `selection_source_matches` / `selection_evaluations`（含 tenant_id、raw 溯源、租约字段） |
| `profit.go` + `profit_test.go` | 落地成本/利润模型（汇率×物流×佣金×退货率×固定成本），含完整单测（正常/边界/非法参数/建议售价） |
| `scoring.go` | LLM 打分：走现有 `providers/ai.Gateway` + `ai_prompts` 可配置 Prompt（code=`selection_scoring`），JSON 容错解析，AI 不可用时规则兜底 |
| `service.go` | 任务编排：创建任务/候选 → 入队 → 行情 → 1688 同款 → 利润 → 打分 → 排序清单；审核 decision；一键转商品草稿（幂等、继承租户、raw 保留选品溯源） |
| `queue.go` / `worker.go` | Redis LIST 队列 + Worker（租约/心跳/重试/partial 状态），HTTP 不阻塞 |
| `handler.go` / `router.go` / `dto.go` / `params.go` | REST API 与参数（系统 settings selection 分组默认值 + 每任务覆盖） |

新 Provider（`backend/internal/providers/`）：

| 目录 | 说明 |
| --- | --- |
| `marketprice/` | 海外在售价 Provider 抽象 + mock（确定性真实感数据）；人工导入价格优先 |
| `sourcematch/` | 1688 同款匹配抽象（图搜+关键词）：`mock.go`（真实感数据）、`crawler.go`（collector 爬虫兜底，有 1688 登录态 profile 时可用，无登录态优雅降级）、`open1688.go`（官方 API 空壳，返回 unavailable） |
| `fx/` | 汇率 Provider（固定汇率表，可 settings 配置） |
| `logistics/` | 物流报价 Provider（首重+续重费率，可配置） |

接线：`internal/api/router.go`（注册路由与 Provider）、`cmd/server/main.go`（启动 Selection Worker）、`internal/database/migrate.go`（自动建表）、`internal/config/config.go`（`SELECTION_*` 配置）。

### Admin 前端（React / Ant Design Pro）

| 文件 | 说明 |
| --- | --- |
| `admin/src/pages/Selection/Tasks/index.tsx` | 选品任务列表 + 新建任务（人工导入/关键词、汇率/佣金/退货率/最低利润率参数）+ 失败重试 |
| `admin/src/pages/Selection/Detail/index.tsx` | 可上架清单：按 AI 评分排序，展示海外价/1688 同款(价格区间、MOQ、供应商、相似度)/预期利润与利润率/AI 评分与理由；通过/拒绝；一键转商品草稿并链接草稿详情 |
| `admin/src/services/selection.ts` | API service 与类型 |
| `admin/config/routes.ts` | `/selection` 路由组 |

### 文档 / 配置

`docs/api.md`（选品 API 契约）、`docs/provider.md`（新 Provider）、`docs/env.md`、`.env.example`（`SELECTION_QUEUE_ENABLED` / `SELECTION_QUEUE_NAME` / `SELECTION_WORKER_CONCURRENCY` / `SELECTION_TASK_TIMEOUT_SECONDS`）、`docs/PROGRESS.md`。

**未改动** source/procurement 相关目录与表（货源档案由并行线开发）。

## 二、运行方法

```bash
# 1. 基础设施（PostgreSQL + Redis）
docker compose up -d

# 2. 配置 .env（cp .env.example .env；本地单租户开发建议加：
#    ENABLE_DEV_DEFAULT_TENANT=true
#    DEV_DEFAULT_TENANT_ID=1
#    否则登录上下文无租户时任务会被 worker 拒绝）

# 3. 后端（自动迁移建表 + 启动 Selection Worker）
cd backend && go run ./cmd/server

# 4. Admin
pnpm install && pnpm dev:admin   # http://localhost:8000

# 5. 页面：AI 选品 → 选品任务 → 新建任务（人工导入每行"标题,在售价,币种,1688链接"，
#    或关键词每行一个）→ 任务 success 后进入"可上架清单"→ 通过 → 转草稿
```

API（均在 `/api/v1`，Bearer 鉴权）：
`POST/GET /selection/tasks`、`GET /selection/tasks/:id`、`GET /selection/tasks/:id/candidates`、`POST /selection/tasks/:id/retry`、`POST /selection/candidates/:id/decision`、`POST /selection/candidates/:id/to-draft`。

## 三、验证结果

- `go test ./...` 全部通过（利润引擎 `profit_test.go` 覆盖正常/零值/负利润/非法汇率/参数钳制/建议售价等）
- `go build ./...`、`go vet ./...` 通过；`gofmt` 无差异
- `pnpm build:admin` Webpack 编译成功
- 端到端演示：创建任务 → Worker 消费 → 5 候选全部 scored → 清单按评分排序 → 通过 → 转草稿 → 草稿详情页可见（来源平台 selection，raw 保留选品溯源与 1688 链接）

演示截图（随交付附件）：
1. 选品任务列表（success 5/5/0）
2. 新建任务表单（参数可配置）
3. 可上架清单（排序 + 1688 同款 + 利润/利润率 + AI 评分）
4. 通过并转草稿（含确认与成功提示）
5. 商品草稿详情（选品来源溯源）

## 四、已知边界

- **1688 官方 API**：仅空壳 Provider（`open1688.go`），返回 unavailable，不发真实请求；凭证走 settings 加密存储。
- **爬虫兜底**：需 collector 在线且有已登录的 1688 浏览器 profile，且候选已带 1688 链接；否则优雅降级到 mock/跳过，不阻塞任务。
- **mock Provider**：确定性伪随机（同输入同输出），便于演示与测试，非真实行情。
- **LLM 打分**：未配置可用 AI Provider 时自动规则兜底（fallback 标记会写入 evaluation）。
- **租户**：任务/候选/草稿均写入创建者租户；本地无租户上下文需开 `ENABLE_DEV_DEFAULT_TENANT`（仅开发用）。
- **海外热销榜自动抓取**：本期以关键词 + 行情 Provider 形式接入，平台榜单爬取留待 Provider 扩展。
