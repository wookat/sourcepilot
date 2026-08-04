# Provider 扩展机制

TradeMind 通过 Provider 抽象接入第三方和本地能力，避免业务模块直接依赖具体平台或 SDK。

## Provider 类型

```text
AI Provider
Storage Provider
Image Provider
Platform Provider
Collector Provider
```

## AI Provider

用于接入大模型服务。

当前重点：

- **OpenAI**（`openai`）
- **OpenAI-compatible**（`openai_compatible`）
- **DeepSeek**（`deepseek`，Chat Completions）
- **通义千问 / Qwen**（`qwen`，DashScope OpenAI 兼容模式）
- 共享 **`compatclient`** HTTP 实现，各 Provider 负责默认地址、错误码中文化与后续扩展入口
- Prompt 模板、AI 调用记录、标题优化、描述生成、客服建议回复

后续可扩展：

- DeepSeek / Qwen 专属错误码、多模态、Embedding、Rerank、用量统计
- 多 Provider 配置表（`settings.ai_providers`）
- Doubao、Gemini、Claude、Ollama（亦可经 `openai_compatible` 接入）

## Storage Provider

用于接入文件与对象存储。

当前支持或预留：

- local
- S3
- Cloudflare R2
- MinIO
- Tencent COS
- Aliyun OSS

敏感字段必须加密存储并脱敏展示。

## Image Provider

用于接入图片处理能力。

当前支持或预留：

- noop
- remove.bg
- OpenAI Image
- ComfyUI

图片任务应通过任务状态与队列执行，避免长请求同步阻塞。

`translate_image_text` 采用 OCR → 翻译 → 样式分组 → 确定性渲染链路。OCR 配置统一放在「设置 → 图片 AI 设置」，由图片文字翻译任务读取用户配置，不允许在代码中写死 Provider、服务地址或 API Key。当前下拉只显示生产可用 Provider：`ai_vision`（当前 AI 设置中的视觉模型）、`paddleocr`（本地 PaddleOCR 服务）、`aliyun`（阿里云 OCR）与 `tencent`（腾讯云 OCR）。图片文字翻译采用严格 OCR 模式：用户选择哪个 OCR Provider，任务就必须真实调用该 Provider；OCR 未配置、测试未通过、调用失败或未识别到文字时任务直接失败，不会自动切换到其他 OCR。腾讯云 OCR 支持 `GeneralBasicOCR` / `GeneralFastOCR`，SecretId / SecretKey 加密保存且前端仅脱敏展示；返回的 `TextDetections` 会转换为统一 OCR blocks，低于 `ocr_min_confidence` 的文字块会被过滤。任务详情输出 configuredOcrProvider、actualOcrProvider、ocrBlocksCount、ocrAverageConfidence 与错误信息。设置页提供 OCR 真实调用测试，阿里云与腾讯云都会真实调用服务并校验 blocks 与 bbox。文字会先聚合为 `main_title`、`badge`、`bottom_badge` 等 group，再按 `auto` / `title_badge` / `preserve_original` 等模板排版；黑底标签会重绘圆角胶囊背景，普通文本优先局部擦除并继承原图字重、颜色和对齐，不再默认用白色矩形覆盖所有区域。结果需输出 `renderQuality` 评分，低于商用阈值时标记 `success_with_warnings`。

## Platform Provider

用于接入跨境电商平台能力。

Douyin Shop (`douyin_shop`) Phase 3 adds a reusable OpenAPI client under `backend/internal/providers/platform/douyinshop`. Signing, common request construction, `param_json` body handling, response parsing, error mapping, safe request logging, token auto-refresh, and shop-info calibration are centralized in the provider package. Business services should call this client instead of hand-writing signatures or raw OpenAPI requests. Store connection testing and manual shop-info sync now use a real platform-side token refresh response to update `shops` / `shop_auth_tokens`; App Secret, access token, refresh token, and full sensitive raw responses must never be returned to the frontend or written to logs.

Douyin Shop Phase 4 adds category and category-attribute sync using official-doc-checked OpenAPI methods `shop.getShopCategory` (`/shop/getShopCategory`, recursive from `cid=0`) and `product.getCatePropertyV2` (`/product/getCatePropertyV2`, `category_leaf_id`). Category data is cached in `platform_categories` and attributes in `platform_category_attributes`; raw responses are stored for backend diagnostics but omitted from normal frontend views. Product Detail → Listing saves Douyin listing preparation to `product_platform_publish_configs` (`platform=douyin_shop`, `shopId`, `categoryId`, `categoryPath`, `platformAttributes`) instead of mutating collected raw data. Readiness checks validate store authorization, selected leaf category, required attributes, and stale cache warnings. Phase 4 deliberately does not implement Douyin product publishing, image upload, order sync, or inventory sync.

Douyin Shop Phase 5 adds internal product draft → Douyin listing draft mapping. Mapping is implemented in the product service layer and stored on `product_platform_publish_configs` as preview fields (`mappedTitle`, `mappedDescription`, `mappedImages`, `mappedSkus`, `mappedPrice`, `mappedStock`, `mappingWarnings`, `mappingErrors`, `lastMappedAt`). It supports AI title / AI description priority, main/detail image preview with `need_sync` status for external images, category attributes, SKU specs, price/profit checks, stock confirmation, manual adjustment, save, and readiness validation. Phase 5 still does not call Douyin product creation or image upload APIs; Phase 6 should handle Douyin image upload / image service sync through Provider abstractions.

Douyin Shop Phase 6 adds image upload to the Douyin material center before product draft creation. Product listing drafts now keep extended `mapped_images` entries for `mainImages` / `detailImages`: local image id, source URL, Storage URL/key, Douyin `platformImageId` / `platformImageUrl`, upload status, failed error code/message, upload time, processed flag, and sanitized raw response. External images are downloaded with timeout, size cap, format/dimension validation, and SSRF private-network blocking, then written to the current Storage Provider before calling Douyin. Storage-backed images are read server-side from the configured Storage Provider; frontend URLs, tokens, and secrets are not used for platform calls. The provider method is `UploadImage(ctx, shopID, req)` and uses the Phase 3 `douyinshop.Client` with official-doc-checked method `supplyCenter.material.batchUploadImageSync` (`/supplyCenter/material/batchUploadImageSync`), preserving token auto-refresh and safe logs. Phase 6 does not create Douyin products, sync orders, or sync inventory.

Douyin Shop Phase 7 adds platform product draft creation from saved mapping + uploaded images. The provider method is `CreateProductDraft(ctx, shopID, req)` in `douyinshop/product.go`, calling official-doc-checked `product.addV2` with `commit=false` and `start_sale_type=1` so items stay in the Douyin draft box and are not directly listed online. Payload assembly lives in `productpublish/douyin_payload.go` and reads `product_platform_publish_configs` mapped fields only (never collect raw). Publish tasks reuse `product_publish_tasks` with `publishMode=save_as_platform_draft`; success writes `product_publications` / `product_publication_skus`. Failures classify into the failure task center with codes such as `DOUYIN_CREATE_PRODUCT_FAILED`. Phase 7 does not sync orders or inventory.

Douyin Shop Phase 9.1 adds SKU binding calibration after platform draft creation. Provider method `GetProductDetail(ctx, shopID, platformProductID)` in `douyinshop/product.go` calls official-doc-checked **`product.detail`** with `show_draft=true` to read draft-box SKU lines (`spec_prices` / `sell_properties`). Service layer `productpublish/douyin_sku_binding.go` matches local `product_publication_skus` by attrs → spec name+price → similar (ambiguous); never guesses low-confidence binds. APIs: `GET/POST /api/v1/product-publications/:id/douyin/sku-bindings*`.

Douyin Shop Phase 9.2 adds manual SKU binding fallback for `ambiguous` / `unmatched` rows. APIs: `POST /api/v1/product-publication-skus/:id/douyin/bind-sku`, `POST .../unbind-sku`. Manual bind validates platform ownership, product ID, non-empty platform SKU ID, and conflict with other local specs; sets `bindStatus=bound`, `bindConfidence=100`, `bindMessage=手动绑定`. Unbind clears `external_sku_id` and marks `unmatched`. `GET .../sku-bindings` returns cached `platformSkus` candidates and `inventorySyncReady`. Inventory sync blocks until all SKUs are bind-ready (`DOUYIN_SKU_BINDING_REQUIRED`, `DOUYIN_SKU_BINDING_CONFLICT`, etc.). Operation logs: `douyin.sku.binding.manual_bind/unbind/recheck/conflict`. Next: full Douyin end-to-end acceptance.

Douyin Shop Phase 9 adds inventory sync MVP via existing inventory orchestration (`inventory` module). The provider implements `InventorySyncProvider.SyncInventory` in `douyinshop/inventory.go`, calling official-doc-checked `sku.syncStock` with `product_id`, `sku_id`, `stock_num`, and `incremental=false` (full stock snapshot). Sync is gated by `inventory_sync_enabled` in platform open config (default off). Reuses `POST /api/v1/product-publication-skus/:id/sync-inventory`, `POST /api/v1/products/:id/sync-inventory`, `GET /api/v1/inventory-sync/tasks*`, `POST /api/v1/inventory-sync/tasks/:id/retry`, and inventory sync batch APIs. Missing `product_publication_skus.external_sku_id` (platform SKU ID) is not guessed — returns `DOUYIN_SKU_NOT_BOUND`. Failures classify into failure task center (`DOUYIN_INVENTORY_SYNC_FAILED`, `DOUYIN_INVENTORY_PERMISSION_DENIED`, `DOUYIN_INVENTORY_RATE_LIMITED`, etc.). Operation logs: `douyin.inventory.sync.start/success/failed/retry`, `douyin.inventory.sku.failed`. Phase 9 does not implement multi-warehouse stock, auto-replenish, or scheduled auto sync by default.

Douyin Shop Phase 8 adds order sync MVP via existing order sync orchestration (`ordersync` module). The provider implements `OrderSyncProvider.SyncOrders` in `douyinshop/order.go`, calling official-doc-checked `order.searchList` with `page`, `size`, `create_time_start`, and `create_time_end` (unix seconds). **Phase 8.1** auto-paginates per task (default max **5 pages** or **500 orders**); configure `order_sync_max_pages` in platform open settings or pass `maxPages` on `POST /api/v1/shops/:id/sync-orders`. Per-page failures are recorded in task `output.pageErrors`; mixed success yields `partial_success`. Task output includes `totalFetched`, `totalPages`, `successPages`, `failedPages`, `nextCursor`/`nextPage`, `createdOrders`, `updatedOrders`, `matchedItems`, `unmatchedItems`, and `deductedStockItems`. List response `shop_order_list` / nested `sku_order_list` are mapped to neutral `PlatformOrder` snapshots (amounts converted from fen to yuan; buyer nickname masked; encrypted address fields omitted from raw). Sync is gated by `order_sync_enabled` in platform open config (default off). Reuses `order.UpsertSyncedOrders`, `MatchOrderItemsForOrder`, optional `DeductInventoryForOrder`, order exception workbench for unmatched SKU, and failure task center for sync failures. Phase 8 does not call Douyin inventory APIs, after-sale/refund APIs, or scheduled polling by default.

**Phase 10.4 (Release Candidate observability)** does **not** add Prometheus. Production monitoring reuses `GET /health` queue blocks, task center failures/alerts (`sub:douyin_*`), operation logs, product operations dashboard, and Douyin runtime APIs: `GET /api/v1/platform/douyin/health`, `GET .../metrics-summary` (in-process 24h counters), `GET .../release-gate`, `POST .../run-health-check`, plus `production-preflight` / `runtime-status`. E2E scripts: `scripts/douyin-e2e-*` (exit `3` + `blocked_by_real_credentials` without credentials; write requires `ALLOW_DOUYIN_WRITE_TEST=true`). CI job `backend-race` in `.github/workflows/go.yml`. See [`DOUYIN_RELEASE_GATE.md`](DOUYIN_RELEASE_GATE.md).

Goofish (`goofish`，闲鱼) is a beta publish provider under `backend/internal/providers/platform/goofish`. 闲鱼没有面向个人卖家的官方开放 API，因此发布通过自托管的「发布桥接服务」完成：桥接服务持有已登录的闲鱼浏览器会话并执行浏览器自动化发布与商品 ID 回填，本 Provider 只通过 HTTP 调用桥接服务（`GET /health` 登录态检查、`POST /publish` 发布）。店铺授权字段为 Bridge 服务地址 + Bearer Token（敏感，加密存储）。能力：`product_publish`、`shop_info`。发布为串行互斥、受平台风控约束不并发；桥接服务不在本仓库内维护。

当前重点平台：

- Douyin Shop（抖店，真实平台闭环优先）
- Goofish（闲鱼，浏览器自动化桥接，首个已验证真实发布通道）
- TikTok Shop
- Shopee
- Lazada
- Amazon

第二平台（TikTok Shop / Shopee）开放平台商品发布 API 调研（鉴权、必填字段、类目/属性、图片上传、限频、开发者账号申请步骤）见 [`platform-integration.md`](platform-integration.md)。

当前真实平台接入顺序优先跑通抖店，不要把抖店与 TikTok Shop 混用：抖店统一内部标识为 `douyin_shop`，TikTok Shop 仍代表跨境平台。已完成 Phase 1–10.4（Release Candidate）：平台配置、OAuth、Client/签名、类目属性、字段映射、图片上传、平台商品草稿创建、订单同步 MVP、库存同步 MVP、SKU 绑定校准与手动兜底、生产预检/运行状态、可观测性与 E2E 脚本/CI。**真实 E2E 仍为 `blocked_by_real_credentials`**。下一阶段：有凭证环境全链路验收与灰度观察。

主要能力：

- 店铺授权
- 店铺信息
- 订单同步
- 商品刊登
- 库存同步
- 客服消息同步与人工发送

平台 App Secret、Access Token、Refresh Token 等必须加密存储。

## Collector Provider

用于接入商品采集来源。

当前重点：

- 1688
- AliExpress beta
- 自定义规则采集 beta

采集服务必须输出统一商品结构，包括标题、图片、属性、SKU、描述图与 raw 原始数据。

## Source Info Provider（货源报价）

用于抓取/查询货源（1688 offer）的 SKU 价格与库存。

- 接口：`backend/internal/providers/sourceinfo`，`Provider.FetchOffer(ctx, offerID, externalSKUIDs)`。
- 当前实现：`Mock`（确定性伪随机报价，无外部请求），供货源档案 refresh 与切换规则演示。
- 后续可扩展：1688 开放平台报价 API、采集服务回填。

## Trade Provider（1688 下单）

用于采购下单链路。1688 官方 API 暂不可用，本期全部走 mock + 人工下单过渡模式。

- 接口：`backend/internal/providers/trade`，包含 `PreviewOrder / CreateOrder / GetPayStatus / GetOrder / GetLogistics / CancelOrder`。
- 当前实现：`Mock1688`（内存态、无外部请求）。`CreateOrder` 返回 `manual=true`，表示需人工前往 1688 下单，之后通过采购单 API 回填订单号 / 运单号推进状态。
- 后续接入官方 API 时，仅需新增 Provider 实现并在路由装配处替换，业务层无需改动。

## 选品相关 Provider（AI 比价选品引擎）

`backend/internal/providers` 下为选品链路新增四类抽象：

- **Market Price Provider**（`providers/marketprice`）：按平台/国家/关键词取海外在售价。当前实现：`mock`（确定性造数）；人工导入价格优先于 Provider。
- **Source Match Provider**（`providers/sourcematch`）：1688 同款匹配（图搜 + 关键词）。实现：
  - `mock`：确定性生成真实感 1688 货源（价格区间/MOQ/供应商评分）。
  - `crawler`：collector 爬虫兜底，仅当候选带 1688 链接且存在已登录 1688 浏览器 profile 时可用；否则返回 `ErrUnavailable` 优雅降级到 mock。
  - `open1688`：官方 API 空壳，凭证可用前始终 `ErrUnavailable`。
- **FX Provider**（`providers/fx`）：汇率，默认内置固定表，可由 settings `selection` 分组（`fx_rate_usd` 等）覆盖。
- **Logistics Provider**（`providers/logistics`）：物流报价，线性模型 `base + perKG × weight`，参数可配置。
- **FX Rate Provider（报表折算）**（`providers/fxrate`，round93）：报表本位币折算的汇率表抽象 `Provider{Table(ctx, tenantID) (*Table, error)}`，`Table` 含本位币与「1 单位原币 = rate 本位币」的汇率映射（`math/big.Rat` decimal 精度，输出保留两位小数半入舍出）。当前实现：`ManualProvider`（读取 settings `report_currency` 分组的手工汇率表，不接实时汇率 API）；后续接入实时汇率源时新增 Provider 实现即可，报表聚合逻辑无需改动。与选品用 `providers/fx` 相互独立（方向与用途不同）。
- **Tracking Provider**（`providers/tracking`，round91 预留）：物流轨迹抓取抽象 `Provider{Name, SupportsFetch, Fetch(carrierCode, trackingNo)}`；当前仅内置 `manual` provider（`SupportsFetch=false`，不调真实快递 API），轨迹状态由人工编辑物流记录推动订单在途→送达既有流转；后续接入快递100/17TRACK 等真实 API 时实现 `Fetch` 并通过 `orders/:id/shipments/:shipmentId/refresh-tracking` 端点回写。

## 扩展建议

新增 Provider 时建议：

1. 先定义接口和统一数据结构。
2. 再实现具体 Provider。
3. 所有外部请求设置超时。
4. 不在日志中输出密钥。
5. 对错误进行可读归因，便于前端展示和任务重试。
6. 必要时同步更新 README、本文档和相关设置页面。

新增 Provider 前请复制或参考 [provider-template.md](provider-template.md)，并按 [module-map.md](module-map.md) 检查 settings、环境变量、API、前端页面、任务队列和文档联动。
