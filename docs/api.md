# API 契约

本文件记录 TradeMind 后端 API 的公共约定。新增、删除或修改接口时，必须同步检查后端 handler / service / DTO、前端 services / types / 页面，以及本文档。

## 基础约定

- 基础路径：`/api/v1`
- 健康检查：`GET /health`、`GET /healthz`、`GET /api/v1/health`（综合）；`GET /health/live`（存活）；`GET /health/ready`（就绪，DB/Redis/迁移/生产门闸）
- 可观测性（P5 / P5.1 / P5-V，需权限）：`GET /api/v1/observability/overview|http|tasks|providers|security`；`overview` 会返回运行态 `runtimeStatus` 与 telemetry 导出摘要，用于区分 `standard_protocol_ready` / `mock_verified` / `real_backend_deferred` / `export_degraded` / `disabled` / `incomplete`；`GET /api/v1/observability/alerts`；`POST /api/v1/observability/alerts/:id/ack|silence`；内部指标：`GET /internal/metrics`（默认仅内网/本机）
- 鉴权：管理端受保护接口使用 `Authorization: Bearer <token>`
- 返回格式：统一 JSON 响应，核心字段为 `code`、`message`、`data`、`traceId`
- 敏感信息：接口不得返回完整 API Key、Token、Secret、Cookie 或密码
- 未知路由：任何未匹配的路径统一返回 JSON 404 envelope（`code=40401`，`message=接口不存在，请检查请求路径`），不再返回 Gin 裸文本 `404 page not found`
- P7-C3 cursor 列表：Product、Order、Inventory Center、Task Center、Webhook Event、Operation Log 支持 `cursor` + `limit`，响应额外返回 `items`、`nextCursor`、`hasMore`、`limit`；旧 `page` / `pageSize` / `list` / `pagination` 兼容保留。超过深 offset 返回 `pagination_offset_too_deep`；cursor 篡改、跨租户/店铺或筛选变化分别返回 `pagination_cursor_signature_invalid`、`pagination_cursor_scope_mismatch`、`pagination_cursor_filter_mismatch`。P7-C4 隔离 Medium PostgreSQL 六类分页 runtime、Query Plan、N+1、Provider 限流、Permission Cache 失效与 Linux Race 证据已关闭；Load/Soak/Regression 仍 pending P7-V2。

## Webhook 入站（公开，无 JWT）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/webhooks/:platform/:eventType` | 平台 Webhook 接收：限体、`Content-Type: application/json`、签名/时间戳校验、幂等持久化、快速 ACK；异步由 DB 轮询 Worker 处理。开发可用 `platform=internal-test`（需 `WEBHOOK_ENABLE_TEST_VERIFIER=true`）。成功 `message=accepted`，`data.eventId` / `duplicate`。 |

## Webhook 事件（管理端）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/webhook-events` | 受保护的 Webhook 事件列表；支持 `platform`、`status`、`eventType`、`shopId`、`start`、`end`、`cursor`、`limit`。只返回元数据、摘要和状态，不返回 `payloadBody` 或签名原文。 |

签名头：`X-Webhook-Signature` 或 `X-TradeMind-Signature`；时间戳：`X-Webhook-Timestamp` / `X-TradeMind-Timestamp`（unix 秒或 RFC3339）。`internal-test` 签名为 HMAC-SHA256 hex（payload = `"{unix}.{rawBody}"`）。失败码含 `WEBHOOK_SIGNATURE_*`、`WEBHOOK_TIMESTAMP_EXPIRED`、`WEBHOOK_PAYLOAD_TOO_LARGE` 等，**不**成功 ACK。

示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "traceId": "request-id"
}
```

## 认证

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/auth/login` | 管理员登录，支持邮箱或手机号。 |
| `POST` | `/api/v1/auth/logout` | 退出登录，客户端丢弃 token。 |
| `GET` | `/api/v1/auth/profile` | 当前管理员信息（含 `role` / `permissions` / `tenantId`，前端据此判定平台管理员可见性）。 |
| `GET` | `/api/v1/auth/register-config` | 注册行为配置（公开）：`{emailVerifyRequired}`。`AUTH_REGISTER_SKIP_EMAIL_VERIFY=true` 且非 staging/production 时为 `false`，登录页据此隐藏验证码输入。 |
| `POST` | `/api/v1/auth/send-email-code` | 注册验证码（`scene: register`）。**反枚举**：SMTP 未配置对任意邮箱统一返回 503 + 中文引导（不区分是否已注册）；SMTP 可用时，邮箱是否已注册、验证码发送成功与否均返回完全一致的 `200 {ok:true}`（已注册不下发验证码、发送失败仅写操作日志 `email_code.send` / `status=skipped|failed`）；限流为「单邮箱 60s 冷却 + 每小时 5 次」叠加「单客户端 IP 每小时 20 次」，超限统一 `429`；注册免验证开关开启时返回 400 提示无需验证码。 |
| `POST` | `/api/v1/auth/register` | 自助注册：**为该账号新建独立租户**并将其设为该租户 admin（不会落入 tenant 0 平台桶）。默认要求 `code`（6 位邮箱验证码）；`emailVerifyRequired=false` 时 `code` 可省略。 |

`legacy_local_storage` 模式下，access token 携带账号当前 `token_version`，每次请求校验账号存在 / 未软删 / 状态 active / `token_version` 未被提升：改密码、改角色、停用、删除后旧 token 立即 `401`（`AUTH_SESSION_REVOKED` / `AUTH_USER_DISABLED`）。

`secure_session` 模式下（staging/production 强制），无 session 绑定的 legacy JWT 统一返回 `401` + `AUTH_SESSION_BINDING_REQUIRED`，客户端应引导重新登录；迁移说明见 `docs/env.md`。

## 设置

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/settings` | 读取**当前租户**的系统设置（仅本租户行；tenant 0 平台配置不返回，平台默认值仅服务端内部消费）。 |
| `PUT` | `/api/v1/settings` | 保存系统设置，敏感字段必须加密。写入租户一律取自认证上下文；请求体 `tenantId` 与当前租户不一致（含 0）返回 403。item 可选 `clear: true` 强制清空已存值（含加密字段，绕过「空加密值保留旧密钥」语义），用于 AI 配置一键清空等场景。 |
| `POST` | `/api/v1/settings/test-ai` | 经 **AI Gateway** 测试 `settings.ai`（支持 `openai` / `openai_compatible` / `deepseek` / `qwen`）。各服务商 **`{provider}_api_key` / `{provider}_base_url` / `{provider}_model`** 独立存储；可选 JSON：`provider`、`base_url`、`model`、`api_key`（写入当前 provider 对应项；`****` 占位则沿用已保存密钥）、`timeout_sec`，用于**未保存前**用当前表单试连；空 body 仅用库内配置。成功 `data`：`ok`、`message`、`provider`、`model`、`latencyMs`。 |
| `POST` | `/api/v1/settings/test-storage` | 测试 Storage Provider 配置。 |
| `GET` | `/api/v1/settings/report-currency` | 读取当前租户的报表本位币与手工汇率表（settings 分组 `report_currency`，按租户隔离存储）：`{provider: "manual", baseCurrency, rates:[{currency, rate}]}`；`rate` 为十进制字符串，含义为「1 单位原币 = rate 本位币」。租户未配置时返回默认本位币 CNY 与空汇率表。需 `settings.manage`（readonly 403）。 |
| `PUT` | `/api/v1/settings/report-currency` | 保存当前租户的报表本位币与手工汇率表：`{baseCurrency, rates:[{currency, rate}]}`，仅影响本租户的报表/毛利估算折算。校验：本位币为 3 位字母代码；汇率为正十进制数；不允许重复币种或给本位币配汇率；最多 50 条。Provider 固定 `manual`（不接实时汇率 API，接口预留 Provider 抽象）。需 `settings.manage`（readonly 403），写操作日志 `settings.report_currency.update`。 |
| `POST` | `/api/v1/storage/test-public-access` | 上传探针图片并通过匿名 HTTP 验证公网可访问性（HTTPS、`image/*`、无登录跳转）；需 `settings.manage`；失败返回 `STORAGE_PUBLIC_*` 错误码。 |
| `POST` | `/api/v1/settings/storage/public-check` | 同上（P1 别名） |
| `GET` | `/api/v1/settings/storage/public-check/latest` | 最近一次公网测试结果（未执行时 `not_run`） |
| `POST` | `/api/v1/settings/test-image` | 测试 `settings.image` 图片 Provider 配置。可选 JSON：`provider`、`testMode`（`config_only` \| `live`，默认 `config_only`）、`settings`（表单覆盖项，支持未保存先测；脱敏 `****` 占位符会忽略并沿用已保存密钥）。成功 `data`：`ok`、`message`、`provider`、`latencyMs`、`supportedTasks`、`configStatus`。不返回 API Key。 |
| `POST` | `/api/v1/settings/test-ocr` | 测试 `settings.image` 中的 OCR 配置。可选 JSON：`provider`（`ai_vision` / `paddleocr` / `baidu` / `aliyun` / `tencent`）、`settings`（表单覆盖项，支持未保存先测；脱敏密钥占位符会忽略）。`paddleocr` 会用后端生成的测试图调用 OCR 服务，检查连通性、文字 `blocks` 与 `bbox`；成功 `data`：`ok`、`message`、`provider`、`latencyMs`、`blocks`、`bboxOk`。 |

## 图片 AI

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/image/providers` | 图片 Provider 能力矩阵（`status` / `supportedTasks` / 难度等，不含密钥）。 |
| `POST` | `/api/v1/image/tasks` | 创建图片任务；创建时校验 Provider 与 `taskType` 组合。 |
| `GET` | `/api/v1/image/tasks` | 图片任务列表。 |
| `GET` | `/api/v1/image/tasks/:id` | 图片任务详情。 |
| `POST` | `/api/v1/image/tasks/:id/retry` | 重试失败任务。 |
| `GET` | `/api/v1/image/tasks/:id/translate-edit-state` | 图片文字翻译人工编辑态：返回原图、已擦除底图、结果图、图片尺寸与可编辑文字块（译文、排版框、擦除框、样式）。 |
| `POST` | `/api/v1/image/tasks/:id/manual-render` | 图片文字翻译人工兜底渲染：按人工编辑后的文字块重新擦除原文并规则重绘译文，结果上传 Storage Provider 并回写任务为 `success_with_review`。 |
| `GET` | `/api/v1/image/tasks/:id/items` | 任务子项列表（源图→结果图、评分 JSON）。 |
| `POST` | `/api/v1/image/tasks/:id/apply` | 将成功任务结果写入 `product_images`（不覆盖原图）。 |
| `GET` | `/api/v1/image/tasks/monitor` | 队列与任务监控快照。 |
| `POST` | `/api/v1/ai/image/tasks` | 创建 AI 图片任务（与 `/image/tasks` 等价）。 |
| `GET` | `/api/v1/ai/image/tasks` | AI 图片任务列表。 |
| `GET` | `/api/v1/ai/image/tasks/:id` | AI 图片任务详情。 |
| `GET` | `/api/v1/ai/image/tasks/:id/translate-edit-state` | 与 `/image/tasks/:id/translate-edit-state` 等价，用于管理端 AI 图片任务页。 |
| `POST` | `/api/v1/ai/image/tasks/:id/manual-render` | 与 `/image/tasks/:id/manual-render` 等价，用于管理端 AI 图片任务页。 |
| `POST` | `/api/v1/ai/image/task-items/:id/save-to-product` | 将任务子项结果保存为新商品图（`applyMode`: main/detail/marketing/ai_generated）。 |
| `POST` | `/api/v1/ai/image/task-items/:id/set-as-main` | 将任务子项结果设为主图（`is_best_main`）。 |
| `POST` | `/api/v1/ai/image/score` | 同步商品图评分（返回 overall/clarity/cleanliness 等维度）。 |

`translate_image_text`（图片文字翻译）读取「设置 → 图片 AI 设置」里的 OCR 配置：`ai_vision` 使用当前 AI 设置中的视觉模型；`paddleocr` 使用本地 PaddleOCR 服务；`aliyun` 会真实调用阿里云 OCR；`tencent` 会真实调用腾讯云 OCR，支持 `GeneralBasicOCR` 与 `GeneralFastOCR`。该任务采用严格 OCR 模式：配置哪个 OCR Provider 就必须实际调用哪个 Provider；OCR 未配置、配置不完整、调用失败或未识别到文字时任务直接失败，不会自动改用其他 OCR。详情输出会包含 `ocr.provider`、`ocr.apiName`、`ocr.configuredOcrProvider`、`ocr.actualOcrProvider`、`ocr.textBlocksCount`、`ocr.averageConfidence`、`ocr.filteredBlocksCount`、`ocr.errorMessage`、`ocr.blocks`、`ocr.groups`、`layout.layoutTemplate` 与 `renderQuality`。每个 OCR block 会补充 `blockClass`、`standardTranslation` 与 `compactTranslation`；顶层会补充 `blockClassifications`、`eraseBBoxCount`、`layoutBBoxCount`、`badgeCount`、`abnormalBadgeCount`、`backgroundPatchScore`、`overlapScore` 与 `finalQualityStatus` 分级：`success`（商用分≥85）、`success_with_review`（75–84，可下载，保存到商品前建议人工检查）、`failed_render_validation`（<65 或中文残留/溢出/遮挡商品主体等硬失败）。调试输出：`debugOriginalUrl`、`debugMaskUrl`、`debugErasedUrl`、`debugFinalUrl`（对应 original/mask/erased/final.png）。65–74 分同任务内自动质量重试一次（`qualityAutoRetried`）。人工兜底使用 `translate-edit-state` 读取可编辑块，再用 `manual-render` 基于原图/已擦除图重新擦除原文并规则重绘译文；输出会记录 `manualEdit`（baseImage、blocks、editedAt、editedBy、eraseMode 等），任务回写为 `success_with_review`。`layout` 还包含 `eraseMode`、`eraseAreaRatio`、`patchAreaRatio`、`flatFillRatio`、`largePatchDetected`、`retryStrategies`、`simulation` 等渲染诊断；顶层同步输出 `configuredOcrProvider`、`actualOcrProvider`、`ocrBlocksCount`、`ocrAverageConfidence`、`detected_source_blocks`、`translated_blocks`、`rendered_blocks`、`target_language_present`、`source_language_residue`、`overflow_blocks`、`style_mismatch_count`、`patch_area_ratio`、`render_quality_score`、`overall_confidence` 便于任务详情和批量排查。`renderQuality` 包含 `textAppliedScore`、`sourceTextRemovedScore`、`layoutScore`、`styleConsistencyScore`、`readabilityScore`、`productPreservationScore`、`commercialUsabilityScore`、`passed` 与 `warnings`；当出现异常 badge、文字重叠、背景补丁、原文残留、版面失衡或商用评分不达标时，任务会以 `low_quality` 返回，不应推荐保存到商品图片或设为主图/详情图。

## 文件

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/files/upload` | 上传文件。 |
| `GET` | `/api/v1/files` | 文件列表。 |
| `DELETE` | `/api/v1/files/:id` | 删除文件。 |

## 商品

商品及其子资源（SKU、图片、平台配置、AI 优化/应用/撤销、图片同步）的全部写接口均有路由级只读守卫：readonly 或无 `product.write`（AI 应用/撤销为 `ai_text.apply`）权限的账号返回 403，可见性 scope 不能替代该守卫。

手工新建草稿（`POST /api/v1/products`）admin 与 operator 均可用（round88，替代 round83 的仅 admin 口径）：请求体支持可选 `shopId`（归属店铺）。operator 必须传 `shopId` 且店铺属于其授权范围（不传返回 400；不在授权范围返回 404「资源不存在」不泄露存在性；仅 view 授权返回 403），创建成功时在同一事务内写入 `product_platform_publish_configs` 店铺关联，草稿对创建者可见；所有校验发生在写入前，被拒绝的请求零落库。admin 保持现口径：`shopId` 可选，传入时同样建立店铺关联（店铺不存在返回 404）。readonly 仍为路由级 403。见 docs/permission-matrix.md「round88」。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products` | 商品草稿列表；支持 `operationStep`（`collect_review` / `title` / `description` / `images` / `pricing` / `publish_check` / `ready`）筛选，并在列表行返回轻量 `operationProgress` 摘要。 |
| `GET` | `/api/v1/products/listing-list/export.csv?ids=` | 批量导出草稿上架清单 CSV：`ids` 为逗号分隔商品 UUID（去重后 ≤50 个），每草稿一行（列含标题/副标题(AI标题)/描述/类目/价格/币种/主图URL/规格列表/来源链接等，UTF-8 BOM），租户+店铺 scope 与草稿列表一致，任一 id 不在 scope 内返回 404；导出属读操作，readonly 可用。 |
| `POST` | `/api/v1/products` | 创建商品草稿。 |
| `GET` | `/api/v1/products/:id` | 商品详情。 |
| `GET` | `/api/v1/products/:id/operation-progress` | 商品运营进度摘要；只读聚合商品、图片、SKU 与既有发布前检查，不调用平台 API、不创建任务、不修改商品。 |
| `PUT` | `/api/v1/products/:id` | 更新商品草稿。 |
| `DELETE` | `/api/v1/products/:id` | 删除或归档商品。 |
| `POST` | `/api/v1/products/:id/apply-ai-title` | 应用 AI 标题；body 支持 `aiTitle`、`taskId`、`expectedUpdatedAt`、`sourceSnapshotHash`，冲突时返回 `AI_CONTENT_APPLY_CONFLICT`，不会静默覆盖人工修改。 |
| `POST` | `/api/v1/products/:id/undo-ai-title` | 安全撤销最近一次 AI 标题应用；若应用后字段又被人工修改，返回 `AI_CONTENT_UNDO_CONFLICT`。 |
| `POST` | `/api/v1/products/:id/apply-ai-description` | 应用 AI 描述；body 支持 `aiDescription`、`taskId`、`expectedUpdatedAt`、`sourceSnapshotHash`，冲突时返回 `AI_CONTENT_APPLY_CONFLICT`。 |
| `POST` | `/api/v1/products/:id/undo-ai-description` | 安全撤销最近一次 AI 描述应用；若应用后字段又被人工修改，返回 `AI_CONTENT_UNDO_CONFLICT`。 |

**批量 AI 文案（Phase A3.1）**

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/products/ai-text/batches/check` | 创建前检查；返回 `summary` + 每商品×类型 `items`（`ready` / `warning` / `blocked`）。 |
| `POST` | `/api/v1/products/ai-text/batches` | 创建批次；支持 `operationTypes`: `title` / `description`；幂等键 `idempotencyKey`；**不自动应用**。 |
| `GET` | `/api/v1/products/ai-text/batches` | 批次列表。 |
| `GET` | `/api/v1/products/ai-text/batches/:id` | 批次详情 + 复核子项；query `status` 筛选。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/retry-failed` | 重试失败、pending、running 子项（含服务重启后的孤儿项）。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/cancel-pending` | 取消 pending 子项。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/apply-selected` | 批量应用；body `itemIds[]`；逐条冲突保护，`partial_success`。 |
| `POST` | `/api/v1/products/ai-text/batches/:id/undo-applied` | 撤销本批次已应用项。 |
| `POST` | `/api/v1/products/ai-text/items/:id/regenerate` | 单条重新生成。 |
| `POST` | `/api/v1/products/ai-text/items/:id/update-edited-text` | 保存编辑文案。 |
| `POST` | `/api/v1/products/ai-text/items/:id/apply` | 单条应用；冲突 409 + `AI_CONTENT_APPLY_CONFLICT`。 |
| `POST` | `/api/v1/products/ai-text/items/:id/reject` | 放弃建议。 |

设计见 [`BATCH_AI_TEXT_OPERATION_DESIGN.md`](BATCH_AI_TEXT_OPERATION_DESIGN.md)。

### 批量 AI 图片（Phase A3.2）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/products/ai-images/batches/check` | 创建前检查；body 含 `productIds`、`imageIds`、`operationTypes`；返回每图×处理方式 `items`。 |
| `POST` | `/api/v1/products/ai-images/batches` | 创建批次；**不自动应用**；幂等键 `idempotencyKey`。 |
| `GET` | `/api/v1/products/ai-images/batches` | 批次列表。 |
| `GET` | `/api/v1/products/ai-images/batches/:id` | 批次详情 + 复核子项。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/retry-failed` | 重试失败 / pending / running 子项。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/cancel-pending` | 取消 pending 子项。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/apply-selected` | 批量应用；body `itemIds[]`、`applyMode`。 |
| `POST` | `/api/v1/products/ai-images/batches/:id/undo-applied` | 撤销本批次已应用项。 |
| `POST` | `/api/v1/products/ai-images/items/:id/regenerate` | 单条重新处理。 |
| `POST` | `/api/v1/products/ai-images/items/:id/apply` | 单条应用；body `applyMode`；冲突 409。 |
| `POST` | `/api/v1/products/ai-images/items/:id/reject` | 放弃结果。 |

`operationTypes`：`quality_check` / `remove_watermark` / `remove_logo` / `white_background` / `optimize_background` / `translate_text` / `select_best_main`。设计见 [`BATCH_AI_IMAGE_OPERATION_DESIGN.md`](BATCH_AI_IMAGE_OPERATION_DESIGN.md)。

| `POST` | `/api/v1/products/:id/images/select-best-main` | 自动评分并选择最佳主图；JSON `mode`: `score_only` / `recommend` / `auto_set`。 |
| `POST` | `/api/v1/products/:id/sync-images` | 将商品外链图片（如淘宝 alicdn）下载并保存到当前 Storage Provider；JSON `scope`: `all` / `main` / `detail`（默认 `all`）。 |
| `POST` | `/api/v1/pricing/calculate` | 单 SKU 发布价试算（不写入数据库）。 |
| `POST` | `/api/v1/products/:id/pricing/apply` | 对商品 SKU 应用定价规则；`confirm=false` 仅预览，`confirm=true` 更新 `product_skus.price`。 |
| `POST` | `/api/v1/products/pricing/batch-apply` | 批量应用定价规则；需 `productIds` 或 `filters`，空条件须 `confirmAll=true`。 |

`GET /api/v1/products/:id` 商品详情会返回统一商品草稿视图：基础字段 `source`、`sourceUrl`、`title`、`originalTitle`、`aiTitle`、`description`、`aiDescription`、`currency`、`status`；图片字段 `mainImages`、`descriptionImages`；结构字段 `attributes`、`skuGroups`、`skus`；价格 / 库存聚合字段 `costPrice`、`salePrice`、`stock`；采集与发布字段 `collectWarnings`、`publishStatus`；高级调试字段 `raw` / `rawData`。前端普通视图只展示标准字段与 warning，`raw` 仅用于高级详情。

`operationProgress` 统一使用实际数据实时计算：采集结果、标题、描述、图片、价格、通用参数、发布检查、刊登草稿准备。返回字段包括 `completionPercent`、`currentStep`、`currentStepLabel`、`nextActionLabel`、`nextActionKey`、`nextActionUrl`、`completedSteps`、`pendingSteps`、`blockers`、`warnings`、`publishReady`、`updatedAt`。列表摘要只返回完成度、当前步骤、下一步入口、阻断/建议数量和可刊登状态；列表聚合批量读取图片、SKU 与图片任务状态，禁止逐行调用平台或自动创建任务。

`pricing.rule` 支持：`costSource`（`collected` / `manual`）、`manualCostPrice`、`markupType`（`fixed` / `percent` / `multiplier` / `none`）、`markupAmount`、`markupPercent`、`markupMultiplier`、`shippingCost`、`weight`、`shippingCostPerWeight`、`platformCommissionPercent`、`exchangeRate`、`minProfit`、`minMarginPercent`、`minPublishPrice`、`roundingMode`（`none` / `integer` / `.9` / `.95` / `.99` / `9.99` / `19.90`）。试算返回 `landedCost`、`commissionFee`、`estimatedProfit`、`profitMarginPercent`；应用后写入 `product_skus.price` 并写操作日志。

`settings` 分组 **`pricing`**：默认加价方式/比例/倍率、固定运费、按重量运费单价（预留）、平台佣金、最低利润、最低利润率、汇率、尾数、平台覆盖、`batch_max_size`（默认 500）。**不**创建刊登任务、**不**调用平台 API。

发布前检查 `GET /api/v1/products/:id/readiness` 返回兼容字段 `status=ready|warning|blocked`，并新增 `result=passed|warning|failed`，以及用户可见 `statusLabel` / `resultLabel`。每个 `checks[]` 项含 `title`、`message`、`severity`（同 `level`）与 `technicalDetails.rawCode`（内部码，前端默认折叠）。`failed` 阻止创建刊登任务；`warning` 可继续但前端必须人工确认。采集 warning 码（如 `DETAIL_IMAGES_INCOMPLETE`）在后端统一中文化。

**多平台刊登中心（Phase A1.2）**

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products/:id/publish-targets` | 可刊登平台、店铺与能力分级（`real_draft_create` / `local_draft_only` / …） |
| `POST` | `/api/v1/products/:id/publish-targets/check` | 多目标独立预检查；body 含 `targets[]`、`commonConfig`、`targetConfigs` |
| `POST` | `/api/v1/products/:id/publish-targets/create-drafts` | 批量创建刊登草稿；形成 `product_publish_batches` + 子任务；支持 `onlyReady`、`retryFailedOnly` + `batchId` |
| `GET` | `/api/v1/product-publish/targets` | 全局可刊登平台与店铺（批量向导） |
| `POST` | `/api/v1/product-publish/batch-targets/check` | 多商品 × 多目标矩阵预检查；body 含 `productIds[]`、`targets[]`、`commonConfig`、`overrides` |
| `POST` | `/api/v1/product-publish/batch-targets/create-drafts` | 多商品批量创建刊登草稿；`onlyReady`、`includeWarnings` |
| `GET` | `/api/v1/product-publish/batches` | 多商品刊登批次列表（按当前租户过滤） |
| `GET` | `/api/v1/product-publish/batches/:id` | 批次详情与子任务（先校验租户归属，跨租户 404；同租户仅创建者可访问，历史无 `createdBy` 批次兼容） |
| `POST` | `/api/v1/product-publish/batches/:id/retry-failed` | 只重试失败子任务 |
| `POST` | `/api/v1/product-publish/batches/:id/cancel-pending` | 只取消 pending 子任务 |

**批量规模限制（Phase A2.1）**：环境变量 `PUBLISH_BATCH_MAX_PRODUCTS`（默认 100）、`PUBLISH_BATCH_MAX_TARGETS`（默认 20）、`PUBLISH_BATCH_MAX_TASKS`（默认 300，即商品数 × 目标数）。超限时 HTTP 400，message：`本次选择的商品和刊登目标较多，请分批创建刊登草稿。`

**幂等**：`create-drafts` 对相同 admin + 商品 + 目标 + 配置 hash 返回已有活跃批次；任务级 dedup 按 `product + platform + shop + config hash` 跳过已成功项。

**配置校验（Phase A2.2）**：`batch-targets/check` 与 `create-drafts` 校验 `commonConfig` / `overrides`（数值非负、策略枚举、商品 / 平台 / 店铺越权与匹配）。失败时 HTTP 400，`code=40004`（`PUBLISH_CONFIG_INVALID`），`data` 含 `title`、`message`、`technicalDetails.field`。

**`commonConfig` 结构**：嵌套 `price` / `image` / `inventory` / `package` + `remark`（详见 [`MULTI_PLATFORM_PUBLISHING_DESIGN.md`](MULTI_PLATFORM_PUBLISHING_DESIGN.md) §A2.2）。

**`overrides` 结构**：`products`、`platforms`、`shops`、`productTargets` 四层局部覆盖；合并优先级见设计文档。

**数据库**：显式 migration 见 [`docs/PUBLISH_BATCH_MIGRATION.md`](PUBLISH_BATCH_MIGRATION.md)。

详见 [`docs/MULTI_PLATFORM_PUBLISHING_DESIGN.md`](MULTI_PLATFORM_PUBLISHING_DESIGN.md)。

刊登任务 `POST /api/v1/products/:id/publish` 会保存 `product_publish_tasks`，任务字段包括 `productId`、`targetPlatform`、`targetStoreId`、`status`（队列态，兼容旧值）、`publishStatus`（业务态：`draft` / `checking` / `ready` / `publishing` / `success` / `failed` / `cancelled`）、`publishMode`、`title`、`description`、`images`、`skus`、`price`、`currency`、`checkResult`、`platformPayload`、`platformResult`、`errorCode`、`errorMessage`、`createdAt`、`updatedAt`。平台字段映射快照包含 `platformTitle`、`platformDescription`、`platformImages`、`platformSkus`、`platformPrice`、`platformStock`、`platformCategory`、`platformAttributes`。

## AI

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/ai/title-optimize` | AI 标题优化（同步/任务，见实现）。 |
| `POST` | `/api/v1/ai/description-generate` | AI 描述生成。 |
| `GET` | `/api/v1/ai/tasks` | AI 任务列表。 |
| `GET` | `/api/v1/ai/tasks/:id` | AI 任务详情。 |

**AI 调用失败的结构化错误码（errorCode）**

`POST /api/v1/products/:id/ai/optimize-title`、`POST /api/v1/products/:id/ai/generate-description`、`POST /api/v1/settings/test-ai` 在 AI Gateway / Provider 调用失败时返回 HTTP 400，`message` 为中文原因；当失败可分类时 `data.errorCode` 返回以下之一（无法分类时省略，成功路径响应结构不变）：

| errorCode | 含义 |
| --- | --- |
| `AI_NOT_CONFIGURED` | 未配置 base_url 或 API Key |
| `AI_INVALID_KEY` | API Key 无效或未授权（上游 401） |
| `AI_FORBIDDEN` | 无权限访问该模型（上游 403） |
| `AI_MODEL_NOT_FOUND` | 模型不存在或无权限 |
| `AI_BAD_BASE_URL` | base_url 不可访问或接口路径错误 |
| `AI_QUOTA_EXCEEDED` | 请求过于频繁或额度受限（上游 429） |
| `AI_UPSTREAM_ERROR` | 服务商 5xx 异常 |
| `AI_TIMEOUT` | 请求超时 |
| `AI_BAD_RESPONSE` | 响应格式不兼容 |

客服 AI 回复建议见 **`POST /api/v1/customer/conversations/:id/ai/generate-reply`**（非 legacy `/ai/chat`）。

### 客服会话子资源 scope 口径（round70）

- 会话全部子资源读写路径（`/customer/conversations/:id/messages` 读写、`/customer/conversations/:id/ai-suggestions` 读写、`mark-replied`、`ai/generate-reply`、`send-platform-message`，以及 `reply-suggestions/:id` 与 `ai-suggestions/:id` 的建议操作）先按父会话的 **tenant + 店铺 scope** 校验归属，与会话详情接口口径一致（同订单/采购/运营任务）。
- 越权/跨租户一律 **404**（不泄露存在性），不再返回 200 空数据；正常授权路径行为与 DTO 不变。
- 客服消息同步同口径：`POST /shops/:id/sync-customer-messages` 按 tenant+店铺 scope 校验店铺；`/customer/message-sync/tasks` 列表按租户过滤（带 `shopId` 时叠加店铺 scope），`tasks/:id` 与 `tasks/:id/retry` 越权 404。新建同步任务写入店铺所属 `tenant_id`。
- 同类收口（父资源 tenant scope，越权 404）：`GET /products/:id/skus/:skuId/inventory-logs`、`GET /products/:id/publication-skus`、`GET /products/:id/ai/tasks`。

### 业务子资源 scope 口径（round71）

round70 复扫清单本轮全部收口，子资源先校验父资源 tenant（+店铺）归属，越权/跨租户统一 **404**（不泄露存在性），正常授权路径行为与 DTO 不变：

- sourcing：`GET /products/:id/sources`、`GET /product-source-skus/:id/price-history` 先校验父商品（价格历史沿 source SKU → product source → product 链）tenant scope。
- imagetask：`GET /image/tasks/:id/items`、`DELETE /image/tasks/:id/items/:itemId` 先校验父任务关联商品的 tenant scope（无商品关联的存量任务无租户归属，保持与任务详情一致的可见性）。
- aioperationbatch：`GET /ai/batches/:id`、`GET /ai/batches/:id/tasks`、`POST /ai/batches/:id/retry-failed`、`POST /ai/batches/:id/apply-results` 按批次创建人所属租户校验（批次无租户列；无创建人的存量批次按租户 0 归属）。
- productpublish：`GET /products/:id/publications` 先校验父商品 tenant，再按店铺 scope 过滤发布行；`GET /product-publications/:id/douyin/sku-bindings`（及 sync/绑定/解绑写路径）先校验发布记录父商品 tenant + 店铺 scope。
- ordersync：`POST /shops/:id/sync-orders` 先校验店铺 tenant + 店铺 scope（与 GET/retry 一致）；新建同步任务写入店铺所属 `tenant_id`。

### AI 批次租户口径（round72）

- `ai_operation_batches` 新增 `tenant_id` 列；创建批次写入当前租户，存量按 `created_by` 所属租户 backfill（推导不出保持租户 0，不放大可见性）。
- `GET /api/v1/ai/batches` 列表按当前租户过滤；`GET /ai/batches/:id`、`GET /ai/batches/:id/tasks`、`POST /ai/batches/:id/retry-failed`、`POST /ai/batches/:id/apply-results` 按 `tenant_id` 列校验（未 backfill 的 tenant-0 行回退按创建人租户），跨租户统一 **404**，不泄露存在性；正常授权路径行为与 DTO 不变（`tenant_id` 不出现在响应中）。

### 刊登批次租户口径与越权 404 统一（round81）

- `product_publish_batches` 新增 `tenant_id` 列（默认 0，索引 `idx_publish_batches_tenant_created`）；创建批次（单商品 `create-drafts` 与多商品 `batch-targets/create-drafts`）写入当前租户，存量按 `created_by` 所属租户 backfill（`migrateRound81PublishBatchTenant`，推导不出保持租户 0，不放大可见性），口径与 round72 `ai_operation_batches` 一致。
- `GET /product-publish/batches` 列表按 `ApplyTenantScope` 过滤；`GET /batches/:id`、`POST /batches/:id/retry-failed`、`POST /batches/:id/cancel-pending` 及 `retryFailedOnly` 重试回放按 `tenant_id` 列校验（未 backfill 的 tenant-0 行回退按创建人租户），跨租户统一 **404**；DTO 不变（`tenant_id` 不出现在响应中）。
- 发布任务越权口径统一：`POST /product-publish/tasks/:id/retry|cancel|recover` 与批次 `retry-failed`/`cancel-pending` 对跨租户/不存在对象由 400 改为 **404**（不泄露存在性）；`recover` 增加租户归属前置校验。同租户业务校验错误仍为 400，同租户非创建者仍为 403。

### 客服话术模板（round109）

租户级客服快捷回复话术模板，供会话回复框一键插入（支持 `{订单号}`、`{买家昵称}`、`{物流单号}`、`{商品名}`、`{店铺名}` 变量占位，插入时按会话上下文自动填充；插入后仍可编辑，发送仍需人工确认，不引入自动外发）。分组固定为 `presale` / `aftersale` / `logistics` / `refund` / `other`。写端点复用客服操作权限口径（`adminperm.CanWriteCustomer`），readonly 账号返回 **403**；全部端点按当前租户隔离；写操作记录 operation log（resource=`customer_reply_template`）。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/customer/reply-templates` | 模板列表。query：`group`（分组）、`keyword`（名称/内容模糊）、`enabled`（三态布尔）。返回 `{ list, canWrite }`，按 `groupKey + sortOrder` 排序。 |
| `POST` | `/api/v1/customer/reply-templates` | 新建模板。body：`groupKey`、`name`、`content`（≤4000 字符）、可选 `sortOrder`（缺省追加到组尾）、`enabled`（默认启用）。 |
| `PUT` | `/api/v1/customer/reply-templates/:id` | 更新模板（支持部分字段：改名、改内容、换分组、启停、排序）。跨租户/不存在返回 **404**。 |
| `DELETE` | `/api/v1/customer/reply-templates/:id` | 删除模板（软删除）。返回 `{ ok: true }`。 |
| `POST` | `/api/v1/customer/reply-templates/reorder` | 组内重排。body：`groupKey`、`ids`（该组完整有序 ID 列表，校验归属后按顺序写 `sortOrder`）。 |

## Dev / Demo 种子（非 production）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/dev/demo-seed/full-project-edge-cases` | **仅 dev/demo 环境**；需 **admin** 权限。写入订单 partial_success、库存同步失败、客服发送失败等样本；不调用真实外部平台。production 禁用。 |

## 采集

权限与 scope 口径（与订单/选品一致）：

- 全部写端点（创建任务/批次、重试、重试失败、打开登录浏览器）路由级挂 `adminperm.RequireWritable`，readonly 账号返回 **403**。
- 任务/批次的读端点按当前租户 `tenant_id` 过滤；跨租户对象访问返回 **404**（不泄露存在性）。`check-login` / `auth-status` 为登录态检测（诊断读），不做 readonly 拦截。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/collect/tasks` | 创建采集任务。`source=custom` 时若 URL 属于已有 **available/beta** 专用采集器域名，返回业务码 **40002**，`data.errorCode=CUSTOM_COLLECT_PROVIDER_CONFLICT`，含 `recommendedProvider` 与 `message`。 |
| `GET` | `/api/v1/collect/tasks` | 采集任务列表。 |
| `GET` | `/api/v1/collect/tasks/:id` | 采集任务详情。 |
| `POST` | `/api/v1/collect/tasks/:id/retry` | 重试采集任务（重试会重置 `queuedAt`，处理超时重新计时）。 |
| `POST` | `/api/v1/collect/rules/ai-generate` | AI 根据商品 URL 生成自定义采集规则（分析页面摘要 → AI → 校验 → 自动规则测试）。1688 / AliExpress 等 **available/beta** 专用平台返回 **40002**。规则非法返回 **40003** `AI_RULE_INVALID`。 |
| `POST` | `/api/v1/collect/rules/ai-generate-and-save` | 同上并直接保存为 `collect_rule`。 |
| `GET` | `/api/collector/providers/1688/auth-status` | 1688 采集浏览器登录态检测（同 `/api/v1/collector/...`）。 |
| `POST` | `/api/collector/providers/1688/open-login-browser` | 打开持久化 Playwright 浏览器供 1688 手动登录。 |
| `GET` | `/api/collector/providers/pinduoduo/auth-status` | 拼多多登录态检测（兼容 GET；内部走 check-login 逻辑）。 |
| `POST` | `/api/v1/collect/providers/pinduoduo/check-login` | 拼多多登录态检测（推荐）。body 可选 `{ "url": "商品详情链接", "testUrl": "设置页检测链接" }`；检测优先级：body.url → 最近失败任务 URL → 设置 `collect_pinduoduo_auth_check_url` → 仅 pifa 首页（`homepage_only`）。 |
| `POST` | `/api/collector/providers/pinduoduo/check-login` | 同上（`/api/collector` 别名）。 |
| `POST` | `/api/collector/providers/pinduoduo/open-login-browser` | 打开拼多多采集浏览器手动登录；body 可选 `{ "url": "商品或 pifa 链接" }`（勿传无参 `mobile.yangkeduo.com` 首页）。 |
| `POST` | `/api/v1/collect/providers/taobao_tmall/check-login` | 淘宝/天猫登录态检测（批量采集开始前也会调用）。body 可选 `{ "url": "商品详情链接", "testUrl": "设置页检测链接" }`；未登录返回业务错误文案；需安全验证时阻止批量开始。 |
| `POST` | `/api/collector/providers/taobao_tmall/check-login` | 同上（`/api/collector` 别名）。 |
| `POST` | `/api/collector/providers/taobao_tmall/open-login-browser` | 打开淘宝/天猫采集浏览器手动登录；body 可选 `{ "url": "商品链接" }`。 |

`GET /api/collector/providers/1688/auth-status` 返回示例：

```json
{
  "provider": "1688",
  "status": "ok",
  "loggedIn": true,
  "needVerification": false,
  "message": "1688 登录态正常",
  "lastCheckedAt": "2026-05-20T12:00:00.000Z",
  "profilePath": "/path/to/collector/data/browser-profiles/1688"
}
```

`status` 取值：`ok`（已登录）、`not_logged_in`（需要登录）、`wechat_auth_required`（微信扫码）、`app_redirect`（App 引导页）、`verification_required`（需验证）、`homepage_only`（仅首页可访问，无法确认登录）、`unknown`（暂时无法确认）。

collector 代理端点（登录态检测 / 打开采集浏览器）在采集服务未启动或不可达（连接拒绝、超时、DNS 失败）时统一返回 HTTP **502**、业务码 **50302**（`CodeCollectorUnreachable`）与中文引导 message（「采集服务未启动或不可达…」）；collector 自身返回的业务错误仍为 502 + 50000 并保留原 message。前端设置中心采集页据 50302 渲染引导态而非报错。

拼多多 `check-login` 返回扩展字段（无 Cookie/HTML）：`profileKey`（`pinduoduo`）、`checkedUrl`、`finalUrl`、`accessStatus`、`urlType`（`wholesale_detail` | `goods_detail` | `homepage` | `app_redirect` | `unknown`）、`checkMode`、`evidence`（`hasProductTitle` / `hasPrice` / `hasMainImage` 等）。**仅当打开商品详情页且识别到标题/价格/主图之一，且无登录/微信/App 引导时** 才返回 `ok`；**pifa 首页可访问不判已登录**。

`POST open-login-browser` 与 `check-login` 使用同一 **`pinduoduo` Profile**（与 1688、custom 隔离）。采集浏览器登录窗口 **1280×900**。

采集任务 DTO 新增向后兼容字段 `queuedAt`（最近一次入队/重试入队时间，旧数据可能为空，为空时以 `createdAt` 计）。任务在 `pending` / `running` / `retrying` 状态停留超过设置项 `collector.collect_task_processing_timeout_seconds`（默认 600 秒，最小 30 秒，后台「采集设置 → 通用采集设置 → 任务处理超时」可改）时，由 task reaper 自动置为 `failed`，`errorMessage` 标注「任务超时」，事件时间线记录 `task.processing_timeout`，可手动重试。

## 店铺与平台

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/shops` | 店铺列表（现行路径；legacy `/stores` 已废弃）。 |
| `GET` | `/api/v1/shops/:id` | 店铺详情。 |
| `POST` | `/api/v1/shops/:id/sync-orders` | 手动触发订单同步。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/refresh` | 刷新抖店授权 Token（示例；各平台 OAuth 见下表）。 |

现行平台 Provider 与开放平台应用配置接口：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/platform/providers` | 返回已注册平台 Provider、能力、状态、`appConfigSchema` 与设置分组。`douyin_shop` 已注册为抖店 / Douyin Shop Provider。 |
| `GET` | `/api/v1/platform/settings/:platform` | 读取平台开放应用配置 schema 与脱敏后的当前值。敏感字段只返回 `****`。 |
| `PUT` | `/api/v1/platform/settings/:platform` | 保存平台开放应用配置。敏感字段加密存储，传入 `****` 表示保留原值。`douyin_shop` 会校验 App Key、App Secret、回调地址、环境与超时时间；发起 OAuth 还需要 `service_id`。 |
| `POST` | `/api/v1/platform/settings/:platform/test-connection` | 测试已保存的平台开放应用配置。`douyin_shop` 应用配置测试校验配置完整性与授权可用性，不做商品 / 订单 / 库存调用。 |
| `GET` | `/api/v1/shops/oauth/douyin/start` | 发起抖店 OAuth；生成 Redis state（10 分钟，绑定管理员、`platform=douyin_shop`、可选 `shopId`），返回 `redirectUrl`。 |
| `GET` | `/api/v1/shops/oauth/douyin/callback` | 抖店授权公开回调；校验 state，处理 `code` / `error`，换取 token，创建或更新 `shops` / `shop_auth_tokens`，成功跳转 `/settings/platforms?platform=douyin_shop&auth=success`。 |
| `GET` | `/api/v1/shops/:id/oauth/douyin/authorize-url` | 已有抖店店铺重新授权，返回 `redirectUrl`。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/refresh` | 使用加密保存的 refresh token 刷新抖店 access token，并用刷新响应校准店铺基础信息；失败时按场景标记 `expired` / `invalid`。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/revoke` | 本地解除抖店授权，清理 / 失效 token，保留历史数据。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/test` | 真实测试抖店店铺连接：检查授权、必要时刷新 token、读取并校准店铺基础信息；不返回 token 明文。 |
| `POST` | `/api/v1/shops/:id/oauth/douyin/sync-shop-info` | 手动同步 / 校准抖店店铺基础信息，复用 Phase 3 OpenAPI Client 与 token 自动刷新能力。 |
| `GET` | `/api/v1/platform/douyin/categories` | 读取本地缓存的抖店类目树；支持 `keyword`、`parentId`、`onlyLeaf`、`refresh=false`、`shopId`（仅 `refresh=true` 时用于手动刷新）。 |
| `POST` | `/api/v1/platform/douyin/categories/sync` | 使用已授权抖店店铺 token 同步类目缓存，body/query 传 `shopId`；写入 `platform_categories`，幂等 upsert。 |
| `GET` | `/api/v1/platform/douyin/categories/stats` | 返回抖店类目缓存数量、叶子类目数量和最近同步时间，供平台开放配置页展示。 |
| `GET` | `/api/v1/platform/douyin/categories/:categoryId/attributes` | 读取某个抖店类目的本地属性缓存；返回必填、可选项、属性值选项和同步时间，不返回 raw。 |
| `POST` | `/api/v1/platform/douyin/categories/:categoryId/attributes/sync` | 使用已授权抖店店铺 token 刷新某个叶子类目的属性缓存，body/query 传 `shopId`；写入 `platform_category_attributes`，幂等 upsert。 |
| `POST` | `/api/v1/platform/douyin/production-preflight` | 抖店上线前生产预检（配置、授权、开关、Storage 公网、数据状态）；body 可选 `{ "liveTest": true }` 对首家已授权店铺做 Token 刷新联调。 |
| `GET` | `/api/v1/platform/douyin/production-preflight/latest` | 读取最近一次预检结果（存于 settings `douyin_preflight.latest_result`）。 |
| `GET` | `/api/v1/platform/douyin/runtime-status` | 读取抖店运行状态（`normal` / `paused` / `emergency_disabled`）、原因与变更时间。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/pause` | 暂停抖店任务；body `{ "reason": "..." }` 必填；记录 `douyin.platform.pause` 操作日志。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/resume` | 恢复抖店运行；body `{ "reason": "..." }` 必填。 |
| `POST` | `/api/v1/platform/douyin/runtime-status/emergency-disable` | 紧急停用；阻止 Worker 调用抖店写接口；body `{ "reason": "..." }` 必填。 |
| `GET` | `/api/v1/products/:id/platform-configs/:platform` | 读取商品的平台刊登准备配置；`douyin_shop` 返回 `shopId`、`categoryId`、`categoryPath`、`platformAttributes`，以及已保存的 `mapping` / `lastMappedAt`。 |
| `PUT` | `/api/v1/products/:id/platform-configs/:platform` | 保存商品的平台刊登准备配置；`douyin_shop` 会校验类目必须为本地缓存中的叶子类目，并记录抖店类目/属性操作日志。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/build-mapping` | 根据当前商品草稿、抖店店铺/类目/属性配置生成并保存抖店刊登草稿预览；不调用抖店创建商品或图片上传接口。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/mapping` | 读取已保存的抖店刊登草稿映射。 |
| `PUT` | `/api/v1/products/:id/platform-configs/douyin_shop/mapping` | 保存人工调整后的抖店刊登草稿映射。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/validate` | 校验抖店刊登草稿映射；可传入临时映射 body，也可不传 body 校验已保存映射。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/images/upload` | 上传当前抖店刊登草稿中的待上传图片到抖店素材中心。body：`imageTypes`（`main` / `detail`）、`retryFailed`、`force`。外链会先下载并写入当前 Storage Provider，再通过后端 Douyin Client 上传；不创建抖店商品。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/images/:imageKey/retry` | 重试单张抖店图片上传。`imageKey` 可用 `localImageId`、`main:0` / `detail:0`、`storageKey` 或已有 `platformImageId`。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/images/status` | 读取当前抖店图片上传状态、Storage 状态、平台图片 ID / URL、失败原因和统计。 |
| `POST` | `/api/v1/products/:id/platform-configs/douyin_shop/create-draft` | 根据已保存抖店映射与已上传素材图创建抖店平台商品草稿。body：`shopId`（必填）、`publishMode`（默认 `save_as_platform_draft`）、`force`（已有 platformProductId 时二次确认）。会先执行发布前检查；`failed` 阻止创建。 |
| `GET` | `/api/v1/products/:id/platform-configs/douyin_shop/publish-tasks` | 列出当前商品的抖店刊登任务（分页）。 |
| `POST` | `/api/v1/product-publish/tasks/:id/cancel` | 取消 pending/running 刊登任务。 |

抖店 SKU 绑定校准与手动兜底（Phase 9.1 / 9.2，`product_publications.id` 或 `product_publication_skus.id` 为路径参数）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/product-publications/:id/douyin/sku-bindings` | 读取当前 `product_publication_skus` 绑定状态汇总（`bound` / `skipped` / `unmatched` / `ambiguous` / `failed` 计数与行明细）；含 `platformSkus` 平台候选、`inventorySyncReady` / `inventorySyncBlockReason`。 |
| `POST` | `/api/v1/product-publications/:id/douyin/sync-sku-bindings` | 调用官方 `product.detail`（`show_draft=true`）拉取抖店 SKU 列表并校准本地映射，回写 `external_sku_id`、`bindStatus`、`bindConfidence`、`bindMessage`、`lastSyncedAt`；更新 `product_publications.skuBindingSyncedAt` 与 `raw_data.platformSkus` 缓存。已绑定 SKU 跳过；多候选标记 `ambiguous` 不强行绑定。 |
| `POST` | `/api/v1/product-publication-skus/:id/douyin/bind-sku` | 人工绑定抖店 SKU。body：`platformSkuId`（必填）、`platformSkuName`、`bindReason`（如 `manual`）。校验 publication 归属 `douyin_shop`、平台商品 ID 存在、SKU ID 非空、不与其他本地规格冲突；覆盖旧绑定时记录操作日志。成功后 `bindStatus=bound`、`bindConfidence=100`、`bindMessage=手动绑定`。 |
| `POST` | `/api/v1/product-publication-skus/:id/douyin/unbind-sku` | 解除绑定。body：`reason`（如 `manual_unbind`）。清空 `external_sku_id`，`bindStatus=unmatched`、`bindMessage=已手动解除绑定`。 |

错误码：`DOUYIN_PRODUCT_DETAIL_FAILED`、`DOUYIN_PRODUCT_NOT_FOUND`、`DOUYIN_PRODUCT_DETAIL_PERMISSION_DENIED`、`DOUYIN_SKU_BINDING_SYNC_FAILED`、`DOUYIN_SKU_BINDING_UNMATCHED`、`DOUYIN_SKU_BINDING_AMBIGUOUS`、`DOUYIN_SKU_MANUAL_BIND_FAILED`、`DOUYIN_SKU_MANUAL_UNBIND_FAILED`、`DOUYIN_PLATFORM_SKU_ID_MISSING`、`DOUYIN_SKU_BINDING_CONFLICT`、`DOUYIN_SKU_BINDING_REQUIRED`。

操作日志：`douyin.sku.binding.manual_bind`、`douyin.sku.binding.manual_unbind`、`douyin.sku.binding.recheck`、`douyin.sku.binding.conflict`（不记录 token / secret）。

抖店库存同步（Phase 9，复用既有 inventory 模块，无新增割裂路径）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/products/:id/publication-skus` | 商品详情库存 Tab 读取刊登 SKU 映射与 `inventorySyncCapability`（`douyin_shop` 为 `beta`）。 |
| `POST` | `/api/v1/product-publication-skus/:id/sync-inventory` | 单 SKU 库存同步；body：`stock`、`options`、`fromInventoryAlert`。要求 `product_publications.external_product_id` 与 `product_publication_skus.external_sku_id` 已绑定。 |
| `POST` | `/api/v1/products/:id/sync-inventory` | 单商品多 SKU 库存同步；body：`shopId`、`skuIds[]`、`options`。 |
| `GET` | `/api/v1/inventory` | 库存中心 SKU 列表（F3）；筛选 stockStatus / skuBindStatus / syncStatus / hasException 等。 |
| `GET` | `/api/v1/inventory/alerts` | 库存预警列表。 |
| `GET` | `/api/v1/inventory/effects` | 订单库存扣减/回滚影响（扣减记录页数据源）。 |
| `GET` | `/api/v1/inventory/logs` | 本地库存变更流水；行内附带 `productId` / `productSkuId` / `productTitle` / `skuCode` / `skuName` / `refOrderNo`（若有）便于溯源。 |
| `GET` | `/api/v1/inventory-sync/tasks` | 库存同步任务列表。 |
| `GET` | `/api/v1/inventory-sync/tasks/:id` | 任务详情。 |
| `POST` | `/api/v1/inventory-sync/tasks/:id/retry` | 重试 failed 任务。 |
| `POST` | `/api/v1/inventory-sync/batches` | 批量库存同步（默认低并发）。 |

Provider 调用官方 `sku.syncStock`（`incremental=false` 全量更新）；受 `inventory_sync_enabled` 开关控制（默认关闭）。缺失平台 SKU ID 或 `bindStatus=unmatched/failed` 返回 `DOUYIN_SKU_BINDING_REQUIRED`；`bindStatus=ambiguous` 返回 `DOUYIN_SKU_BINDING_AMBIGUOUS`；绑定冲突返回 `DOUYIN_SKU_BINDING_CONFLICT`；不猜测同步。库存同步前须全部 SKU 处于可同步绑定状态（bound / skipped 且已有 `external_sku_id`）。

### P9 Inventory Sync Backend API（Batch 5）

Batch 5 的 fixture/mock-only 后端 API 使用 `/api/v1/inventory-sync`，复用现有认证、租户上下文、RBAC、审计和签名 keyset cursor。所有写请求必须带 `Idempotency-Key`；JSON body 必须为受限 `application/json`，拒绝未知字段和多余 JSON 值。该 API 不接收凭证、不调用真实 Douyin、不读写真实平台库存，也不启动 worker/cron/queue。

| Method | Path | Permission | Purpose |
| --- | --- | --- | --- |
| `POST` | `/api/v1/inventory-sync/runs` | `inventory_sync.run` | Create a fixture-backed sync run |
| `GET` | `/api/v1/inventory-sync/runs` | `inventory_sync.read` | Signed keyset run history |
| `GET` | `/api/v1/inventory-sync/runs/:runId` | `inventory_sync.read` | Safe run detail/statistics/error summary |
| `POST` | `/api/v1/inventory-sync/runs/:runId/rerun` | `inventory_sync.rerun` | Guarded retry of a failed/cancelled retryable run |
| `GET` | `/api/v1/inventory-sync/runs/:runId/snapshots` | `inventory_snapshot.read` | Immutable snapshot list and result filter |
| `GET` | `/api/v1/inventory-sync/snapshots/:snapshotId` | `inventory_snapshot.read` | Immutable snapshot detail |
| `GET` | `/api/v1/inventory-sync/bindings` | `sku_binding.read` | Tenant-scoped binding list |
| `GET` | `/api/v1/inventory-sync/bindings/:bindingId` | `sku_binding.read` | Safe binding detail |
| `GET` | `/api/v1/inventory-sync/bindings/:bindingId/history` | `sku_binding.read` | Calibration/manual decision history |
| `GET` | `/api/v1/inventory-sync/snapshots/:snapshotId/calibrations` | `sku_binding.read` | Versioned calibration candidates |
| `POST` | `/api/v1/inventory-sync/snapshots/:snapshotId/recalibrate` | `sku_binding.manage` | Idempotent controlled new calibration version |
| `GET` | `/api/v1/inventory-sync/manual-binding-requests` | `sku_binding.read` | Pending/status manual request list |
| `GET` | `/api/v1/inventory-sync/manual-binding-requests/:requestId` | `sku_binding.read` | Request and immutable decisions |
| `POST` | `/api/v1/inventory-sync/manual-binding-requests/:requestId/confirm` | `sku_binding.resolve_manual` | Revision-checked manual confirmation |
| `POST` | `/api/v1/inventory-sync/manual-binding-requests/:requestId/reject` | `sku_binding.resolve_manual` | Revision-checked manual rejection |
| `GET` | `/api/v1/inventory-sync/runs/:runId/audit-events` | `inventory_sync.audit.read` | Allowlisted tenant-scoped audit timeline |

List endpoints return `{items, nextCursor, hasMore, limit}` and never expose offset/page totals. DTOs intentionally omit raw provider cursors, checkpoints, payloads, credential fields, and idempotency hashes.

通用刊登任务接口（含抖店）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/product-publish/tasks` | 刊登任务列表 |
| `GET` | `/api/v1/product-publish/tasks/:id` | 任务详情（含 `platformPayload` 平台提交内容、`platformProductId` 抖店商品 ID、`retryable` 是否可重试） |
| `POST` | `/api/v1/product-publish/tasks/:id/retry` | 重试 failed 任务 |

`product_platform_publish_configs.mapped_images` 在抖店 Phase 6 保存扩展结构：

```json
{
  "mainImages": [
    {
      "localImageId": "",
      "sourceUrl": "",
      "storageUrl": "",
      "storageKey": "",
      "platformImageId": "",
      "platformImageUrl": "",
      "imageType": "main",
      "uploadStatus": "pending|processing|uploaded|failed|skipped",
      "errorCode": "",
      "errorMessage": "",
      "uploadedAt": "",
      "processed": false
    }
  ],
  "detailImages": []
}
```

抖店 OAuth / Client / 类目 / 映射 / 图片错误码：`DOUYIN_APP_CONFIG_INCOMPLETE`、`DOUYIN_OAUTH_STATE_INVALID`、`DOUYIN_OAUTH_DENIED`、`DOUYIN_OAUTH_CODE_MISSING`、`DOUYIN_TOKEN_EXCHANGE_FAILED`、`DOUYIN_TOKEN_REFRESH_FAILED`、`DOUYIN_SHOP_INFO_FAILED`、`DOUYIN_AUTH_EXPIRED`、`DOUYIN_PERMISSION_DENIED`、`UNKNOWN_DOUYIN_AUTH_ERROR`、`DOUYIN_API_ERROR`、`DOUYIN_RATE_LIMITED`、`DOUYIN_REQUEST_TIMEOUT`、`DOUYIN_RESPONSE_PARSE_FAILED`、`UNKNOWN_DOUYIN_ERROR`、`DOUYIN_CATEGORY_SYNC_FAILED`、`DOUYIN_CATEGORY_EMPTY`、`DOUYIN_CATEGORY_NOT_SELECTED`、`DOUYIN_CATEGORY_NOT_LEAF`、`DOUYIN_CATEGORY_ATTR_SYNC_FAILED`、`DOUYIN_REQUIRED_ATTR_MISSING`、`DOUYIN_CATEGORY_CACHE_STALE`、`DOUYIN_CATEGORY_PERMISSION_DENIED`、`DOUYIN_TITLE_MISSING`、`DOUYIN_TITLE_TOO_LONG`、`DOUYIN_DESCRIPTION_MISSING`、`DOUYIN_DESCRIPTION_NEEDS_REVIEW`、`DOUYIN_MAIN_IMAGE_MISSING`、`DOUYIN_MAIN_IMAGE_NOT_UPLOADED`、`DOUYIN_MAIN_IMAGE_UPLOAD_FAILED`、`DOUYIN_DETAIL_IMAGE_UPLOAD_PARTIAL_FAILED`、`DOUYIN_IMAGE_NEED_UPLOAD`、`DOUYIN_IMAGE_UPLOAD_EXPIRED`、`DOUYIN_IMAGE_NEED_SYNC`、`DOUYIN_DETAIL_IMAGE_EMPTY`、`DOUYIN_DETAIL_IMAGE_NEED_SYNC`、`DOUYIN_ATTR_VALUE_INVALID`、`DOUYIN_SKU_MISSING`、`DOUYIN_SKU_PRICE_INVALID`、`DOUYIN_SKU_STOCK_UNCONFIRMED`、`DOUYIN_SKU_ATTR_INCOMPLETE`、`DOUYIN_PRICE_MISSING`、`DOUYIN_PRICE_INVALID`、`DOUYIN_PROFIT_TOO_LOW`、`DOUYIN_STOCK_UNCONFIRMED`、`DOUYIN_STOCK_INVALID`、`DOUYIN_COLLECT_NEEDS_REVIEW`、`IMAGE_URL_NOT_ACCESSIBLE`、`IMAGE_DOWNLOAD_FAILED`、`IMAGE_READ_FAILED`、`IMAGE_FORMAT_UNSUPPORTED`、`IMAGE_SIZE_TOO_LARGE`、`IMAGE_DIMENSION_INVALID`、`IMAGE_PROCESS_FAILED`、`STORAGE_UPLOAD_FAILED`、`DOUYIN_IMAGE_UPLOAD_FAILED`、`DOUYIN_STORE_NOT_AUTHORIZED`、`DOUYIN_CREATE_PRODUCT_FAILED`、`DOUYIN_PRODUCT_PAYLOAD_INVALID`。API 错误响应 `data.errorCode` 返回业务码；callback 失败通过 `reason` query 返回。所有响应均不得返回 App Secret、access token 或 refresh token 明文。

## 抖店可观测性 / Health & Metrics（Phase 10.4）

> **不** 提供 Prometheus `/metrics`。抖店生产监控复用进程健康、任务中心、操作日志与运营看板。E2E 脚本见 `scripts/douyin-e2e-*`；门禁见 [`DOUYIN_RELEASE_GATE.md`](DOUYIN_RELEASE_GATE.md)。

### 进程健康（含抖店相关队列）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/health` | 匿名；`data.status` 为 `up` / `degraded`；含 `checks.database`、`checks.redis` |
| `GET` | `/healthz` | 同上（k8s/监控探针惯用别名） |
| `GET` | `/api/v1/health` | 同上 |

`data` 中与抖店 Worker 相关的块（队列启用时）：

| 字段 | 说明 |
| --- | --- |
| `orderSyncQueue` | 订单同步 Redis 队列深度、Worker 并发、`redisAvailable` |
| `productPublishQueue` | 商品刊登（含抖店草稿创建）队列 |
| `inventorySyncQueue` | 库存同步（含 `sku.syncStock`）队列 |
| `workers` | 各 Worker 心跳；`degraded=true` 时整体 `status=degraded` |

### 抖店运行态、健康与指标（Phase 10.4，无 Prometheus）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/platform/douyin/runtime-status` | `normal` / `paused` / `emergency_disabled`、原因与时间 |
| `GET` | `/api/v1/platform/douyin/health` | 抖店聚合健康：`overallStatus`（`healthy` / `degraded` / `unhealthy` / `disabled`）、`config` / `auth` / `storage` / `tasks` / `api` 分区、`grayRelease`、`runtime`；快照写入 settings `health_snapshot` |
| `GET` | `/api/v1/platform/douyin/metrics-summary` | 滚动 24h 内存指标（API 成功率/耗时、Token 刷新、任务 stale、刊登/订单/库存/SKU 计数等）；**非** Prometheus `/metrics` |
| `GET` | `/api/v1/platform/douyin/release-gate` | Release Candidate 门禁清单：`overallConclusion`（默认 `Release Candidate`）、`items[]`（`key` / `label` / `status` / `message`）；`credentials` 项在无真实 E2E 时为 `blocked` |
| `POST` | `/api/v1/platform/douyin/run-health-check` | 执行健康聚合 + taskcenter 抖店告警 scan；返回与 `GET .../health` 相同结构并持久化快照 |
| `POST` | `/api/v1/platform/douyin/production-preflight` | 上线预检；`data.blockedByRealCredentials` 为 true 时表示无真实凭证 |
| `GET` | `/api/v1/platform/douyin/production-preflight/latest` | 最近一次预检 JSON |

### 任务中心（失败 / 告警 / 摘要）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/task-center/summary` | 失败任务与告警计数摘要 |
| `GET` | `/api/v1/task-center/failures` | 失败任务列表；`taskType` 含 `ai_text`（批量 AI 文案子项）；深链 `detailUrl` → `/product/ai-text-batches/:id?itemId=` |
| `GET` | `/api/v1/task-center/failures/:taskType/:id` | 失败详情（脱敏 raw） |
| `GET` | `/api/v1/task-center/alerts` | 站内告警列表 |
| `POST` | `/api/v1/task-center/alerts/scan` | 扫描并生成告警（dedupe） |
| `POST` | `/api/v1/task-center/alerts/:id/notify` | Webhook 通知（需配置） |
| `GET` | `/api/v1/task-center/failure-categories` | 含 `sub:douyin_*` 分类 |

### 运营任务（Operation Task）

详见 [`P8_OPERATION_TASK_API.md`](P8_OPERATION_TASK_API.md)。权限与 scope 口径（与订单/采购/异常一致，round61）：

- 运营任务为店铺维度业务数据（`operation_tasks.shop_id`，可空=租户级）。admin 不受限；operator/readonly 仅见已授权店铺任务，无授权店铺列表为空；租户级任务（`shop_id IS NULL`）仅 admin 可见。
- 越权/跨租户直读（含 drafts/approvals/attempts/events 子资源与全部写路径）统一 **404**，不泄露存在性。
- 创建接受可选 `shopId`：admin 可省略（租户级）；非 admin 必须绑定已授权店铺（缺失 400；未授权/跨租户店铺 404）。存量数据按 `source_reference`（店铺 id / 唯一店铺关联的商品 id）迁移时自动 backfill，推导不出的保持租户级。
- execute/retry 适配器 payload 校验失败返回 HTTP **400**、业务码 **40001**、`data.errorCode=execution_validation_failed`（round61 前误为 500/50000）；失败 attempt 记录行为不变。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/operation-tasks` | 创建运营任务（可选 `shopId`） |
| `GET` | `/api/v1/operation-tasks` | 任务列表（keyset cursor；店铺 scope） |
| `GET` | `/api/v1/operation-tasks/:taskId` | 任务详情 |
| `POST` | `/api/v1/operation-tasks/:taskId/cancel` | 取消 |
| `POST` | `/api/v1/operation-tasks/:taskId/drafts` | 创建初始草稿 |
| `PATCH` | `/api/v1/operation-tasks/:taskId/drafts/latest` | 编辑最新草稿 |
| `GET` | `/api/v1/operation-tasks/:taskId/drafts` | 草稿历史 |
| `POST` | `/api/v1/operation-tasks/:taskId/approve` | 审批通过 |
| `POST` | `/api/v1/operation-tasks/:taskId/reject` | 审批驳回 |
| `POST` | `/api/v1/operation-tasks/:taskId/execute` | 执行（安全适配器） |
| `POST` | `/api/v1/operation-tasks/:taskId/retry` | 手动重试 |
| `GET` | `/api/v1/operation-tasks/:taskId/attempts` | 执行 attempt 历史 |
| `GET` | `/api/v1/operation-tasks/:taskId/events` | 审计事件历史 |

### 操作日志与运营看板

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/operation-logs` | 只读；权限 `operationlog.view`，租户 scope + 审计店铺 scope（`shop_id IS NULL` 的日志按租户可见，有店铺归属的仅授权店铺可见；业务数据的空授权=空结果语义不变）；筛选 `action`/`username`/`resource`/`start`/`end`（管理端 `/system/operation-logs` 同名参数 URL 深链）；不返回 Secret/Token。登录审计：登录成功与已知账号的失败尝试记入该账号所属租户；未知账号的失败尝试与锁定/限流拒绝（发生在账号查询之前）保留 tenant 0 作为平台级安全审计 |
| `GET` | `/api/v1/dashboard/product-operations` | 运营总览 KPI、漏斗、异常（只读 DB 聚合，不调平台 OpenAPI；含 RBAC 店铺 scope） |
| `GET` | `/api/v1/dashboard/overview` | 模块化 overview + 10 张运营卡片 |
| `GET` | `/api/v1/dashboard/todos` | 统一待办流（P0/P1/P2 优先级） |
| `GET` | `/api/v1/dashboard/health` | 子系统健康 + 配置风险摘要 |

### AI 商品运营工作台（Phase A3.3）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/ai/operation-workbench/summary` | 待办统计卡片（文案/图片/发布检查/刊登异常/今日已处理） |
| `GET` | `/api/v1/ai/operation-workbench/todos` | 分页待办列表；支持 `type` / `priority` / `platform` / `shopId` / `keyword` / 时间 |
| `GET` | `/api/v1/ai/operation-workbench/todos/:id` | 单条待办详情 |
| `POST` | `/api/v1/ai/operation-workbench/todos/refresh` | 重新聚合待办（只读，不写库、不调平台 API） |

### 用户与权限管理（admin only）

均要求 `user.manage` 权限（仅 admin 角色具备）；只读账号写操作返回 403。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/admin/users` | 分页用户列表（`role` / `status` / `keyword`）；非 admin 用户附 `storePermissions`（含 `storeName`） |
| `GET` | `/api/v1/admin/users/:id` | 用户详情 |
| `POST` | `/api/v1/admin/users` | 创建用户（邮箱/手机号 + 初始密码 + 角色） |
| `PATCH` | `/api/v1/admin/users/:id` | 修改显示名 / 角色 / 状态；不能禁用自己、不能自我降级 |
| `PUT` | `/api/v1/admin/users/:id/store-permissions` | 整体替换店铺授权（admin 角色无需分配） |
| `POST` | `/api/v1/admin/users/:id/reset-password` | 重置用户登录密码（`{"password"}`，至少 6 位）；递增 `token_version` 并吊销该用户全部 secure 会话/refresh token，旧会话下次请求即失效且不可续期；写操作日志 `user.password.reset` |
| `DELETE` | `/api/v1/admin/users/:id` | 软删除用户（`deleted_at`，数据保留）；同时撤销全部店铺授权、递增 `token_version` 并吊销该用户全部 secure 会话/refresh token（旧会话立即不可续期）；不能删除当前登录账号（400）；路由级只读守卫 403 |

### 平台租户管理（仅平台管理员）

平台管理员口径（最保守判定）：**当前登录账号 `tenant_id = 0` 且角色为 `admin`**。其他租户的 admin、operator、readonly 一律 403；开租操作写入操作日志（`tenant.create`，不含密码）。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/platform/tenants` | 租户列表（含隐式平台租户 0），每项返回 `id` / `name` / `status`（`active` / `disabled`）/ `adminCount` / `createdAt` |
| `POST` | `/api/v1/platform/tenants` | 创建租户 + 初始管理员（事务一次建好）。请求体 `{name, adminEmail, adminPassword}`；返回 `{tenant, adminId, adminEmail}`。租户名 / 管理员邮箱重复返回 400；初始管理员登录后即为新租户 admin |
| `PUT` | `/api/v1/platform/tenants/:id` | 租户改名。请求体 `{name}`；重名 400、租户不存在 404、平台租户（id 0）不可改名 400。写操作日志 `tenant.rename` |
| `POST` | `/api/v1/platform/tenants/:id/disable` | 停用租户。停用后该租户所有账号登录被拒（错误码 `AUTH_TENANT_DISABLED`，中文提示「租户已被停用」），已有会话在下次请求（access 校验 / refresh 轮换）时失效；平台租户（id 0）不可停用 400、不存在 404。写操作日志 `tenant.disable` |
| `POST` | `/api/v1/platform/tenants/:id/enable` | 启用租户，恢复该租户账号登录；不存在 404。写操作日志 `tenant.enable` |
| `POST` | `/api/v1/platform/tenants/:id/purge` | 清退删除租户（后台任务）。请求体 `{confirmName}` 必须与租户名称完全一致；前置条件租户已停用（未停用 400）；tenant 0 永不可清退（400）；不存在 404；同租户已有进行中任务 400。提交后后台级联删除该租户全部业务数据（账号、店铺、商品/草稿、货源、订单、采购、库存、客服、选品、采集、发布、批次及该租户业务操作日志）并逐表校验零残留；平台侧开租/清退审计保留在 tenant 0（`tenant.purge.start` / `tenant.purge.done` / `tenant.purge.failed`）。返回清退任务 `{id, tenantId, tenantName, status, createdAt}` |
| `GET` | `/api/v1/platform/tenants/:id/purge` | 查询该租户最近一次清退任务状态（`pending` / `running` / `succeeded` / `failed`）；成功时附逐表零残留报告 `report.tables`（表名 → 残留行数，全部为 0）与 `report.total`；无任务 404 |

租户停用生效口径（round82）：登录（legacy / secure session）、refresh 轮换、每次带 Bearer 的请求（session 令牌走 `ValidateSessionAccess`，legacy 令牌由中间件按 claims 租户检查）都会检查用户所属租户状态，租户 `disabled` 时统一返回 401 `AUTH_TENANT_DISABLED`；tenant 0（平台租户）与无 `tenants` 行的 legacy 租户恒为 active。租户删除仅通过清退流程（round89）：先停用，再由平台管理员输入租户名确认后提交后台清退任务；清退不可恢复，业务日志随租户一并删除（保留策略：租户内业务审计随租户生命周期终止），平台侧开租/清退审计与清退任务记录（`tenant_purge_tasks`）永久保留在 tenant 0。

## AI 比价选品引擎 API

候选商品 → 海外在售价 → 1688 同款匹配 → 落地成本/利润模型 → LLM 打分 → 可上架清单。均需 Bearer 认证。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/selection/tasks` | 创建选品任务（items 人工导入 / productIds 草稿 / keywords），入 Redis 队列异步处理 |
| `GET` | `/api/v1/selection/tasks` | 分页任务列表，支持 `status` 过滤；返回候选/打分/失败计数 |
| `GET` | `/api/v1/selection/tasks/:id` | 任务详情 |
| `GET` | `/api/v1/selection/tasks/:id/candidates` | 可上架清单：候选 + 1688 同款 + 利润评估 + AI 评分，按评分降序 |
| `POST` | `/api/v1/selection/tasks/:id/retry` | 失败/部分成功任务重新入队 |
| `POST` | `/api/v1/selection/candidates/:id/decision` | 人工审核 `{"decision":"approved\|rejected"}` |
| `POST` | `/api/v1/selection/candidates/:id/to-draft` | 已通过候选一键转商品草稿（幂等，重复调用返回已有草稿） |

数据表：`selection_tasks` / `selection_candidates` / `selection_source_matches` / `selection_evaluations`。
利润参数（汇率、佣金、物流、退货率等）默认读 settings `selection` 分组，可按任务 `params` 覆盖。

## P6 Backup / Restore / Release / DR API

All P6 write operations require Bearer authentication and backend RBAC. The frontend never receives shell commands, full backup paths, storage secrets or database credentials.

`/api/v1/ops/*` 作用于**整个部署**（全库备份/恢复、发布、容灾演练），因此为**平台租户专属**：仅 `tenant_id = 0` 的 admin 可访问，业务租户任何角色一律 403（除下表权限位外的额外前置守卫）。

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/ops/backups` | `backup.read` | 备份记录列表；不返回完整对象路径。 |
| `POST` | `/api/v1/ops/backups` | `backup.create` | 创建备份任务；未启用备份时生成待复核记录。 |
| `GET` | `/api/v1/ops/backups/:id` | `backup.read` | 备份详情。 |
| `POST` | `/api/v1/ops/backups/:id/verify` | `backup.verify` | 执行备份校验；未启用加密时加密检查按「未启用（跳过）」处理；`details.checks` 返回结构化检查项。 |
| `GET` | `/api/v1/ops/backups/:id/download` | `backup.download` | 流式下载校验通过的 completed 备份；readonly/operator 403；备份不存在或越权 404；写入操作日志。 |
| `POST` | `/api/v1/ops/backups/:id/hold` | `backup.hold` | 添加手动保留。 |
| `DELETE` | `/api/v1/ops/backups/:id` | `backup.delete` | 删除非运行、非 hold 的备份记录。 |
| `GET` | `/api/v1/ops/restores` | `restore.read` | 恢复验证列表。 |
| `POST` | `/api/v1/ops/restores` | `restore.execute` | 创建隔离恢复验证；production 目标默认拒绝。 |
| `GET` | `/api/v1/ops/restores/:id` | `restore.read` | 恢复验证详情。 |
| `POST` | `/api/v1/ops/restores/:id/verify` | `restore.verify` | 恢复验证（本地/开发限定，production 拒绝）；真实执行备份文件完整性与 `pg_restore --list` 结构校验，其余检查项在 `details.checks` 中标注 `not_implemented`。 |
| `GET` | `/api/v1/ops/releases` | `release.read` | 发布记录列表。 |
| `POST` | `/api/v1/ops/releases` | `release.create` | 创建发布记录和 manifest 摘要。 |
| `GET` | `/api/v1/ops/releases/:id` | `release.read` | 发布详情。 |
| `POST` | `/api/v1/ops/releases/:id/execute` | `release.execute` | 执行受控发布状态机。 |
| `POST` | `/api/v1/ops/releases/:id/rollback` | `release.rollback` | 应用层回滚；禁止自动数据库恢复。 |
| `GET` | `/api/v1/ops/dr/status` | `dr.read` | 灾备状态与 Deferred 项。 |
| `POST` | `/api/v1/ops/dr/drills` | `dr.execute` | 执行隔离演练（本地/开发限定，production 拒绝）；必须确认隔离环境并提供 backupId；真实执行备份文件完整性与 `pg_restore --list` 结构校验，其余项在 `reportJson.checks` 中标注 `not_implemented`。 |

P6-VR closure evidence is recorded in `docs/P6_VR_FINAL_CLOSURE_REPORT.md`: isolated restore, isolated release rollback, Linux race, and final gates passed. P6 still does not mark Production Ready and does not perform real production restore, PITR drill or traffic switch.

## P7 Performance / Capacity API Status

P7 currently adds backend configuration, database tables, local rate-limit middleware, guarded dataset / load / soak / race scripts and validation gates, but does **not** expose public management APIs yet. P7-V has real isolated Medium dataset evidence (`insertedRows=1,900,150`, `failedRows=0`), while load, soak, regression and final closure remain incomplete and must not be described as production performance verification.

Planned ops routes remain design-only until implemented with RBAC, re-authentication for writes and audit logging:

| 方法 | 路径 | 状态 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/ops/performance/overview` | planned | 聚合 API / DB / Worker / Provider 性能概览。 |
| `GET` | `/api/v1/ops/performance/regressions` | planned | 性能回归记录。 |
| `GET` | `/api/v1/ops/capacity/overview` | planned | 数据规模、连接池、Worker 容量与扩容建议。 |
| `GET` | `/api/v1/ops/rate-limits` | planned | 限流策略只读展示，不暴露 Redis key 或明文 PII。 |
| `PUT` | `/api/v1/ops/rate-limits/:policyId` | planned | 高权限、重认证、审计后修改受控策略。 |
| `GET` | `/api/v1/ops/quotas` | planned | Tenant / Shop / User / System 配额模板。 |
| `POST` | `/api/v1/ops/profiling/cpu` | planned | 内部高权限 profiling，duration 有上限，不返回任意路径。 |

Current code-level P7 endpoints affected: product and order list APIs reject excessive deep offset via P7 pagination guard; HTTP requests can be locally rate-limited when `RATE_LIMIT_ENABLED=true`.

## 商品-货源档案（sourcing）

一品多源的供应商与货源档案，供采购协同与后续 AI 比价选品引擎引用。所有路由走统一 JWT 鉴权与统一返回结构。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/suppliers` | 供应商列表（keyword/status/分页）。 |
| `POST` | `/api/v1/suppliers` | 创建供应商（platform 默认 `1688`）。 |
| `PUT` | `/api/v1/suppliers/:id` | 更新供应商。 |
| `DELETE` | `/api/v1/suppliers/:id` | 删除供应商（存在绑定货源时 409）。 |
| `GET` | `/api/v1/products/:id/sources` | 商品的货源列表（含供应商与 SKU 映射）。 |
| `POST` | `/api/v1/products/:id/sources` | 绑定货源；`supplierName` 不存在时自动建供应商；首个货源自动设为主供应商；重复绑定 409。 |
| `PUT` | `/api/v1/product-sources/:id` | 更新优先级 / 锁定 / 状态 / MOQ / 备货周期等。 |
| `POST` | `/api/v1/product-sources/:id/set-primary` | 人工切换主供应商，写入 `source_switch_events`。 |
| `POST` | `/api/v1/product-sources/:id/sku-mappings` | 批量保存本地 SKU ↔ 外部 SKU 映射；价格变化写 `source_price_history`。 |
| `GET` | `/api/v1/product-source-skus/:id/price-history?days=90` | 历史进价（默认 90 天）。 |
| `DELETE` | `/api/v1/product-source-skus/:id` | 删除单条 SKU 映射（软删除）；删除后该映射不再参与采购单生成与采购受阻判定。 |
| `POST` | `/api/v1/products/:id/sources/refresh` | 通过 Source Info Provider（当前 mock）刷新价格/库存，并按切换规则处理断货/涨价；`alerts` 为结构化对象数组（`code` + `sourceId` + `supplierName` + `reason` + `thresholdPercent`，code 取值 `fetch_failed / price_increase / primary_locked / no_backup / switch_suggested / auto_switched`），由前端渲染中文文案。 |
| `GET` | `/api/v1/source-switch-events?productId=` | 货源切换审计（auto / manual / suggested）；suggested 事件带处理状态（open / adopted / ignored）。 |
| `POST` | `/api/v1/source-switch-events/:id/adopt` | 采纳一条待处理的切换建议：主供应商切换为建议的备选货源并标记 adopted，写操作日志；非待处理建议返回 409。 |
| `POST` | `/api/v1/source-switch-events/:id/ignore` | 忽略一条待处理的切换建议（标记 ignored），写操作日志。 |
| `GET` | `/api/v1/product-source-alerts` | 预警货源总览：当前处于涨价预警/断货状态的货源（含商品标题、供应商、是否主供应商、该商品待处理建议数）。 |
| `GET` | `/api/v1/product-sources/orphans` | 孤儿货源列表：关联商品已被（软）删除的货源（含原商品标题、供应商、SKU 映射数）；这些货源会阻塞供应商删除。 |
| `DELETE` | `/api/v1/product-sources/:id` | 解绑孤儿货源（软删除货源及其 SKU 映射，写操作日志）；仅限关联商品已删除的货源，商品仍存在时返回 409；解绑后对应供应商可删除。 |

切换规则：`priority` 越小越优先；主货源断货且未锁定时自动切换到最优可用备用源并记 `auto` 事件；涨价超过阈值（settings `sourcing` 组，默认 10%）仅生成 `suggested` 建议事件，不自动切换；`locked` 货源不参与自动切换。

## 采购协同（procurement，人工下单过渡模式）

1688 官方 API 暂不可用；当前通过 Trade Provider mock + 人工下单模式流转。状态机：`draft → pending_confirm → placing → placed → paid → shipped → delivered`，另有 `failed / cancelled / voided`；非法流转返回 400。`voided`（作废）仅允许从终态/错误态（`delivered / failed / cancelled`）进入，用于处置测试单或错误单据；作废单保留审计与操作日志，但不再参与统计/待办/生成防重覆盖判定（口径与 cancelled/failed 一致）；已入库库存不自动回滚。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/procurement/orders/generate` | 从销售订单按主货源供应商聚合生成采购单（draft）；未匹配 SKU / 缺主货源等以 `blockers` 返回，每条含 `orderId`/`code`/`message`，`source.missing`、`mapping.missing` 额外返回 `productId` 与 `localSkuId`（新增可选字段，向后兼容，供前端生成直达链接）；映射无参考价时自动回退到最近历史进价，仍缺价以 `warnings`（`price.missing`）返回；支持 `idempotencyKey` 幂等；明细行已被未取消/未失败/未作废采购单覆盖时跳过并以 `warnings`（`line.covered`）返回，取消或作废原采购单后可重新生成。 |
| `GET` | `/api/v1/procurement/orders?status=&salesOrderId=` | 采购单列表；`salesOrderId` 按来源销售订单过滤（订单详情「关联采购单」用），非法 UUID 返回 400。 |
| `GET` | `/api/v1/procurement/cost-estimates/:id` | 销售订单成本/毛利估算（id 为销售订单）：按主货源 SKU 映射参考价（缺价回退最近历史进价）逐行估算 CNY 成本；订单币种为 CNY、报表手工汇率表（`report_currency`，与销售报表同一口径，优先）可折算 CNY→订单币种、或配置了 `settings.pricing.exchangeRate`（CNY→订单币种，兜底）时折算 `estimatedCost/grossProfit/marginPercent`，任一行缺价时不计算毛利，问题行以 `issueCode`（`sku.unmatched`/`source.missing`/`mapping.missing`/`price.missing`）返回。 |
| `POST` | `/api/v1/procurement/cost-estimates/batch` | 批量成本/毛利估算（订单列表用）：body `{"orderIds": ["..."]}`（≤50 个），返回 `items`（orderId → 汇总：`estimatedCostCny/exchangeRate/estimatedCost/grossProfit/marginPercent/missingLines`），不存在的订单被省略。 |
| `GET` | `/api/v1/procurement/orders/:id` | 详情（items / events / logistics）。 |
| `GET` | `/api/v1/procurement/orders/:id/export.csv` | 导出采购清单 CSV（含 1688 链接、外部 SKU、数量、参考价，UTF-8 BOM）。 |
| `GET` | `/api/v1/procurement/purchase-lists/export.csv?ids=` | 批量导出合并采购清单 CSV：`ids` 为逗号分隔采购单 UUID（去重后 ≤50 个），逐单合并明细行（「采购单号」列区分来源），任一 id 不存在返回 404。 |
| `POST` | `/api/v1/procurement/orders/:id/submit` | draft → pending_confirm（经 Provider PreviewOrder）。 |
| `POST` | `/api/v1/procurement/orders/:id/confirm` | pending_confirm → placing（记录确认人/时间，调用 mock CreateOrder，人工模式）。 |
| `POST` | `/api/v1/procurement/orders/:id/mark-placed` | 回填 1688 订单号，placing → placed。 |
| `POST` | `/api/v1/procurement/orders/:id/mark-paid` | 人工标记付款，placed → paid。 |
| `POST` | `/api/v1/procurement/orders/:id/logistics` | 回填运单号/承运商，paid → shipped。 |
| `POST` | `/api/v1/procurement/orders/:id/mark-delivered` | shipped → delivered；同事务将各明细数量加回本地 SKU 库存并写 `inventory_change_logs`（`purchase_inbound`，按 business_event_key 幂等）。 |
| `POST` | `/api/v1/procurement/orders/:id/retry` | failed → placing。 |
| `POST` | `/api/v1/procurement/orders/:id/cancel` | 取消（终态前均可）。 |
| `POST` | `/api/v1/procurement/orders/:id/void` | 作废：`delivered / failed / cancelled → voided`，body `{reason?}`；写 `purchase_order_events` 与操作日志（`procurement.void`）；已入库库存不自动回滚；仅可写角色（readonly 403）。 |
| `PUT` | `/api/v1/procurement/orders/:id/items/:itemId/price` | 补填/修改明细参考价：`{expectedPrice}`（>0），仅 draft / pending_confirm 状态可改，重算采购单 `totalAmount` 并返回详情。 |
| `POST` | `/api/v1/procurement/orders/batch-mark-placed` | 批量回填 1688 订单号：`{items:[{purchaseOrderId, externalOrderId}]}`，单批 ≤200 行，逐行独立处理返回 `{succeeded, failed, results[]}`（部分成功不回滚）。 |
| `POST` | `/api/v1/procurement/orders/batch-logistics` | 批量回填运单号：`{items:[{externalOrderId, trackingNo, carrier?}]}`，按 1688 外部订单号匹配采购单（placed 状态会先自动 mark-paid），返回逐行结果。 |

所有状态流转写入 `purchase_order_events`；对应管理端页面为 `/procurement/orders`。

范围口径：全部采购协同接口按当前租户过滤；非管理员进一步限制到被授权店铺（采购单经明细行来源销售订单的 `shop_id` 判定，无店铺授权列表为空）。范围外的采购单/销售订单 ID 一律返回 404（不泄露存在性），批量接口逐行按「不存在」处理或省略。

### 销售订单批量导入（人工建单过渡）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/orders/import` | 批量创建手工销售订单：`{orders:[CreateBody], matchSkus?}`，单批 ≤200 张；订单号已存在（库内或批内重复）返回 `skipped_duplicate` 不重复建单，单张失败不影响其余；`matchSkus=true` 时创建后自动按 SKU 编码匹配本地规格。返回 `{total, created, duplicate, failed, results[]}`（含逐单 `itemsMatched`）。手工订单创建（含单张 `POST /orders`）会写入当前租户 `tenant_id`；`CreateBody.shopId` 可选（导入弹窗可选一个店铺应用到整批），仅允许当前账号可见的店铺。 |

`GET /api/v1/orders` 支持可选 `hasPurchase` 过滤（`1`/`true`＝已有未取消/未失败/未作废采购单覆盖任一明细行，`0`/`false`＝无；缺省不过滤），与生成采购单防重的覆盖判定同一口径；首页「订单待采购」待办卡使用 `payStatus=paid&hasPurchase=0` 直达。

对应管理端入口：`/orders` 工具栏「批量导入订单」，粘贴格式 `订单号,客户名,商品标题,SKU编码,数量,单价[,币种]`，同订单号多行合并为多明细。

### 销售订单发货（物流回填与状态流转）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/orders/stats/sales` | 经营概览统计：返回 `{generatedAt, baseCurrency, windows:[{key: today|7d|30d, orderCount, paidCount, shippedCount, paidAmounts:[{currency, amount, orders, baseAmount?}], paidAmountBase, unconvertedCurrencies?}]}`，按创建时间窗口在租户内统计订单数/已付款/已发货与分币种已付款销售额；`paidAmountBase` 为按 `report_currency` 手工汇率表折算入本位币的合计（decimal 精确计算、输出保留两位小数），缺少汇率的币种不计入合计而是列入 `unconvertedCurrencies`（原币桶无 `baseAmount`）；店铺 scope 与订单列表一致（非 admin 按授权店铺过滤）。 |
| `GET` | `/api/v1/orders/stats/daily` | 经营报表按日统计：`?days=30`（默认 30，最大 90），返回 `{generatedAt, days, baseCurrency, items:[{date: YYYY-MM-DD, orderCount, paidCount, shippedCount, paidAmounts:[{currency, amount, orders, baseAmount?}], paidAmountBase, unconvertedCurrencies?}]}`；口径与本位币折算规则与 `stats/sales` 一致（当前租户、软删除订单不计入，已发货口径同 `stats/sales` 的 `shippedCount`），店铺 scope 与订单列表一致（非 admin 按授权店铺过滤）。 |
| `GET` | `/api/v1/orders/stats/daily/export.csv` | 导出经营报表逐日明细 CSV：`?days=30`（默认 30，最大 90），只读端点（readonly 可用），数据、口径与 scope 与 `stats/daily` 完全一致；UTF-8 BOM，列为「日期/订单数/已付款数/已发货数」+ 窗口内出现的每个币种两列「已付款销售额(币种)/折算金额(币种→本位币)」（币种字典序；无汇率时折算列留空而非补 0）+「已付款销售额合计(本位币)/未折算币种」，空日期行补 0。 |
| `GET` | `/api/v1/orders/shipping-list/export.csv?ids=` | 批量导出发货清单 CSV：`ids` 为逗号分隔销售订单 UUID（去重后 ≤50 个），逐单合并明细行（「订单号」列区分来源，含客户名/电话/商品/SKU/数量/币种/金额），「快递单号(回填)」「承运商(回填)」列留空供线下打单后回填批量发货；任一 id 不在租户内返回 404。 |
| `POST` | `/api/v1/orders/shipments/batch` | 批量发货：`{items:[{orderNo, trackingNo, carrier?, carrierCode?}], defaultCarrierCode?}`（≤200 条），按订单号在租户内匹配销售订单并新增 `shipped` 物流（订单自动流转）；R91 起支持第三列物流商（代码/名称/名称前缀均可，仅限已启用物流商）与 `defaultCarrierCode` 默认物流商；旧两列格式兼容（无物流商时沿用「其他快递」）；命中物流商时按其规则宽松校验运单号并自动补轨迹 URL；未付款/已取消/未找到/重复订单号/运单号校验失败逐行失败，返回 `{succeeded, failed, results[]}`；成功行附 `inventoryDeducted`（该订单是否已有成功库存扣减；发货本身不扣库存，仅提示口径）。 |
| `POST` | `/api/v1/orders/:id/shipments` | 新增物流记录：`{carrier, carrierCode?, trackingNo?, trackingUrl?, status?, shippedAt?, deliveredAt?}`；`status` 缺省 `pending`；传 `carrierCode` 时关联租户内已启用物流商（回写 `carrierId` 与名称快照，按物流商规则宽松校验运单号、自动补轨迹 URL）；不传保持自由文本承运商兼容。 |
| `PUT` | `/api/v1/orders/:id/shipments/:shipmentId` | 更新物流记录（同上字段）。 |
| `DELETE` | `/api/v1/orders/:id/shipments/:shipmentId` | 删除物流记录。 |
| `GET` | `/api/v1/orders/print/sheets?ids=` | 拣货/发货单打印数据：`ids` 为逗号分隔销售订单 UUID（去重后 ≤50），返回 `{items:[{orderNo, platform, shopName, customerName/Phone/Email, remark, orderedAt, items[], shipments[]}]}` 供浏览器打印页渲染（人工贴单，非电子面单）；越权/不存在 404，店铺 scope 同订单详情。 |
| `POST` | `/api/v1/orders/:id/shipments/:shipmentId/refresh-tracking` | 轨迹刷新（Provider 预留）：当前仅 `manual` provider，返回 `{provider:"manual", supported:false, message, shipment}` 不调真实 API；轨迹状态仍通过手工编辑物流状态推动订单在途→送达既有流转。 |

### 物流商管理（carriers，round91）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/carriers` | 物流商列表（租户隔离）：`?enabled=1&keyword=`；首次访问自动为租户幂等预置国内常用快递（顺丰/京东/中通/圆通/申通/韵达/邮政EMS/极兔/德邦/其他）；返回 `{items:[{id, code, name, enabled, isPreset, trackingUrlTemplate, sortOrder}]}`。 |
| `POST` | `/api/v1/carriers` | 新增自定义物流商：`{code, name, trackingUrlTemplate?, sortOrder?}`；`code` 限 `^[a-z0-9_-]{1,64}$` 且租户内唯一。 |
| `PUT` | `/api/v1/carriers/:id` | 更新物流商（启停 `enabled`、改名、轨迹模板、排序）；预置物流商可停用。 |
| `DELETE` | `/api/v1/carriers/:id` | 删除自定义物流商；预置物流商不可删除（400），只能停用。 |

运单号校验（宽松）：顺丰 `SF+10~15 位数字`、京东 `JD+10~18 位`、EMS `两字母+9 数字+两字母`；其余物流商统一 `6~40 位字母数字横线`；自由文本承运商（不传 `carrierCode`）不校验，保持旧行为。

物流写入时的自动流转（仅前进、不回退，按订单生命周期 rank 判定）：

- 物流状态 `shipped` / `in_transit` → 订单 `status=shipped`、`fulfillmentStatus=fulfilled`，缺省补 `shippedAt`；
- 物流状态 `delivered` → 订单 `status=delivered`，缺省补 `shippedAt` / `deliveredAt`；
- `pending` / `exception` / `returned` 不触发订单状态变化；已取消/退款/关闭订单不会被回退或改写。

首页待办新增 `order_await_shipment`「订单待发货」（已付款且 `fulfillmentStatus=unfulfilled` 且未发货/关闭的订单数），链接 `/orders/list?payStatus=paid&fulfillmentStatus=unfulfilled`。

### 违禁词合规检测（banned-words，round109）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/banned-words` | 违禁词列表（租户隔离）：`?category=&level=&keyword=&enabled=1`；首次访问自动为租户幂等预置基础库（广告法极限词 / 通用违禁词 / 医疗功效词 / 品牌侵权词）；返回 `{items:[{id, word, category, level, isPreset, enabled, suggestion}]}`。 |
| `POST` | `/api/v1/banned-words` | 新增租户自定义违禁词：`{word, category?, level, suggestion?}`；`level` 为 `forbidden`（禁止，阻断刊登）或 `warning`（警告，仅提示）；词在租户内唯一。 |
| `PUT` | `/api/v1/banned-words/:id` | 更新违禁词；预置词仅可改 `enabled`（启停），词面/类别/级别/建议只读。 |
| `DELETE` | `/api/v1/banned-words/:id` | 删除自定义违禁词；预置词不可删除（400），只能停用。 |
| `GET` | `/api/v1/banned-words/categories` | 分类列表：`{items:[{category, categoryLabel, enabled, wordCount}]}`。 |
| `PUT` | `/api/v1/banned-words/categories/:category` | 分类启停：`{enabled}`；停用后该分类词不参与扫描。 |
| `GET` | `/api/v1/products/:id/banned-words/check` | 扫描单个商品草稿（标题 / AI 标题 / 详情 / AI 描述），返回 `{productId, status(blocked|warning|passed), statusLabel, forbiddenCount, warningCount, hits:[{word, field, fieldLabel, category, categoryLabel, level, levelLabel, suggestion, positions:[{start,end}]}], fields:[{field,label,text}]}`；`positions` 为 Unicode 码点偏移，用于前端高亮。 |
| `POST` | `/api/v1/products/banned-words/check-batch` | 批量扫描：`{productIds}`（单次最多 100），返回 `{list: ScanResult[]}`。 |

租户隔离与权限：越权访问返回 404；写操作（POST/PUT/DELETE）readonly 角色返回 403。发布检查（`/products/:id/readiness`）已接入 `compliance` 分组：禁止级命中产生 `error`（`compliance.banned_word_forbidden`，阻断 `canPublish`），警告级产生 `warning`（`compliance.banned_word_warning`，不阻断）。

### 订单异常工作台：采购受阻（procurement_blocked）

`GET /api/v1/orders/exceptions` 新增聚合异常类型 `procurement_blocked`：已付款、未发货且未取消/退款/关闭的销售订单行，若已绑定本地 SKU 但商品缺可用主货源（`source_missing`）或主货源缺该 SKU 映射（`mapping_missing`），且未被任何未取消/未失败的采购单行覆盖，则以 `sourceType=order_item` 进入工作台。返回体：

- `summary.procurementBlocked`：未处理数量。
- 行内 `sourcingUrl`：`/sourcing/product-sources?productId=<productId>`，用于跳转货源档案绑定主货源/补 SKU 映射。
- 处理/忽略沿用现有 mark 接口（`exceptionType=procurement_blocked`）。

Dashboard 同步：`GET /api/v1/dashboard/product-operations` 的 `summary.procurementBlockedOrderItems`，以及统一待办 `procurement_blocked`（P0，链接 `/orders/exceptions?exceptionType=procurement_blocked`）。货源档案页 `/sourcing/product-sources` 支持 `?productId=` 直达指定商品。

### 订单异常工作台：利润为负（negative_margin）

`GET /api/v1/orders/exceptions` 新增聚合异常类型 `negative_margin`：已付款、未发货且未取消/退款/关闭的销售订单，按主货源参考价成本估算（与 `/procurement/cost-estimates` 同一口径）预估毛利为负时，以 `sourceType=order` 进入工作台（每次列表最多扫描最近更新的 200 个候选订单）。缺参考价或未配汇率导致毛利不可算的订单不会误报。返回体：

- `summary.negativeMargin`：未处理数量。
- 行内 `errorMessage` 含售价、预估成本（CNY）、预估毛利与毛利率；`orderUrl` 直达订单详情复核。
- 处理/忽略沿用现有 mark 接口（`exceptionType=negative_margin`，`sourceType=order`，`sourceId=订单 ID`）。

Dashboard 同步：`summary.negativeMarginOrderCount`，统一待办 `order_negative_margin`（P0，链接 `/orders/exceptions?exceptionType=negative_margin`）。

### 订单异常工作台：全部视图

`GET /api/v1/orders/exceptions` 查询参数除 `handled=true`（只看已处理标记）、`ignored=true`（只看已忽略标记）外，支持 `all=true`：同时返回未处理、已处理与已忽略的行（`summary` 口径不变，仍只统计未处理）。默认（不带三者）只返回未处理行。

## 迁移导入（migrationimport，round92）

从店小秘 / 马帮导出文件迁移存量商品与历史订单的导入向导 API。统一 JWT 鉴权与统一返回结构；写操作（parse/validate/commit）readonly 返回 403。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `POST` | `/api/v1/imports/parse` | multipart 上传（`kind=product|order`、`file`，CSV/XLSX ≤10MB）；返回 `columns / rows / totalRows / fileHash / sourceFormat（dianxiaomi|mabang|custom，按表头自动识别）/ mapping（自动猜列，字段 key → 列下标，-1 表示未映射）/ fields（目标字段定义，含 required）`。单批 ≤1000 数据行，超限 400。 |
| `POST` | `/api/v1/imports/validate` | 请求体 `kind / shopId / columns / rows / mapping / fileName / fileHash / sourceFormat`；只校验不落库。返回 `totalRows / validRows / errorRows / groupCount（商品数或订单数）/ errors[]（rowNumber/field/message，逐行必填缺失、重复、非法值）`。`shopId` 必填且必须是当前账号可操作的店铺（operator 越权返回 404/403）。 |
| `POST` | `/api/v1/imports/commit` | 与 validate 同请求体；确认导入。商品按「商品名称」聚合创建草稿（status=draft，行=SKU，已存在的 SKU 编码按重复跳过）；订单按「订单号」聚合创建（platform=migration，来源状态映射到内部枚举，收件人地址存入 `rawData.receiver` 与备注）。**幂等**：同租户同 kind 同 `fileHash`（文件 sha256）只提交一次，重传原样返回首个批次结果（`replayed=true`）。返回 `jobId / status（success|partial_success|failed）/ totalRows / successRows / failedRows / duplicateRows / replayed`。 |
| `GET` | `/api/v1/imports?kind=&page=&pageSize=` | 导入历史（租户隔离，倒序）；返回 `list[]（ImportJob + errorRowCount）/ total / page / pageSize`。 |
| `GET` | `/api/v1/imports/:id` | 单批详情：`job` + `errorRows[]`（仅持久化失败/重复行：rowNumber/status(failed|duplicate)/field/message/rawValues）。 |
| `GET` | `/api/v1/imports/:id/errors.csv` | 错误行报告下载（UTF-8 BOM CSV：行号/状态/字段/错误信息/原始数据）。 |

订单状态映射（店小秘/马帮 → 内部枚举）：未付款/待付款→`pending`；已付款→`paid`；待处理/待审核/待打单/已打单/配货中→`processing`；已发货→`shipped`；已完成/已签收→`delivered`；已作废/已取消→`cancelled`；已退款→`refunded`；无法识别的状态逐行报错不入库。格式假设与字段别名详见 `docs/migration-guide.md`。

`POST /api/v1/orders` 请求体新增可选字段 `remark`（备注）与 `rawData`（JSON 原始数据），向后兼容。

## 权限矩阵契约（round52）

- 全部已注册路由的「路由 × {admin, operator, readonly, 跨租户}」授权预期登记在
  `backend/internal/securitytests/permmatrix/matrix.json`，由权限矩阵契约测试逐端点断言；
  **新增端点未登记预期时测试失败**。运行方式与登记流程见 `docs/permission-matrix.md`。
- `/api/v1`（登录后）与 `/api/collector` 全部写方法路由（POST/PUT/PATCH/DELETE）挂有
  路由级只读守卫（`adminperm.ReadonlyWriteGuard`）：readonly 账号一律 403，
  纯计算类 POST（calculate/check/preview/validate/estimate）与自助 session 管理除外
  （允许清单见 `backend/internal/pkg/adminperm/write_guard.go`）。

## 修改 API 时的同步要求

- 后端：handler、service、DTO、权限和错误处理一起检查。
- 前端：`admin/src/services`、`admin/src/types`、相关页面字段和状态映射一起检查。
- 文档：同步本文档、`docs/module-map.md` 和必要的 README 能力描述。
- 安全：涉及密钥、Token、密码、Cookie 时同步 `SECURITY.md`。
- 任务：耗时接口必须使用任务状态，不应在 HTTP 请求中长时间阻塞。
## P3.2 Douyin Webhook Routing Addendum

For `platform=douyin_shop` / `douyin`, the public webhook route resolves the verified payload to a concrete shop binding before persistence. Accepted events carry `tenantId`, `internalShopId`, `platformShopId`, `appId`, and `bindingId` into `webhook_events` and downstream order upsert. Duplicate detection is scoped by `platform + tenant_id + platform_shop_id + event_id`, so the same platform `event_id` from two shops does not collide.

Resolution failures are non-success ACKs and may use codes such as `DOUYIN_WEBHOOK_SHOP_NOT_RESOLVED`, `DOUYIN_WEBHOOK_SHOP_AMBIGUOUS`, `DOUYIN_WEBHOOK_BINDING_REVOKED`, `DOUYIN_WEBHOOK_AUTHORIZATION_EXPIRED`, `DOUYIN_WEBHOOK_APP_BINDING_MISMATCH`, and `DOUYIN_WEBHOOK_TENANT_MISMATCH`.
