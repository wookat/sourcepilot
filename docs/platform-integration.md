# 第二平台开放平台调研：TikTok Shop 与 Shopee（虾皮）

> R94 预研文档。目标：为「第二平台」真实刊登接入做准备，梳理两个开放平台商品发布 API 的鉴权模式、商品创建必填字段、类目/属性、图片上传与限频，并给出开发者账号申请步骤（供老板申请）。本轮不接真实 API（无凭证），刊登动线走 `local_draft_only` 降级路径；本文与现有 adapter（`backend/internal/providers/platform/tiktok`、`backend/internal/providers/platform/shopee`）的实现口径对齐。

信息来源：TikTok Shop Partner Center 公开文档（partner.tiktokshop.com/docv2）、Shopee Open Platform 公开文档（open.shopee.com）。文档更新截至 2026-08；上线前需以官方最新文档复核。

## 1. 总览对比

| 维度 | TikTok Shop | Shopee（虾皮） |
| --- | --- | --- |
| 开放平台入口 | partner.tiktokshop.com（Partner Center） | open.shopee.com（Open Platform） |
| 内部平台标识 | `tiktok` | `shopee` |
| settings 分组 | `platform_tiktok` | `platform_shopee` |
| 鉴权模式 | OAuth 2.0 授权码 + app_key/app_secret HMAC-SHA256 请求签名 + shop_cipher | 授权码换 token + partner_id/partner_key HMAC-SHA256 请求签名 + shop_id |
| 商品创建 API | `POST /product/{version}/products`（版本如 202309） | `POST /api/v2/product/add_item` |
| 图片上传 API | `POST /product/{version}/images/upload` | `POST /api/v2/media_space/upload_image` |
| 多规格 | 请求体内 skus + sales_attributes | `init_tier_variation` 二次调用 |
| 沙箱 | Partner Center 提供沙箱环境与 API Testing Tool | Sandbox Testing V2（open.shopee.com 沙箱） |

## 2. TikTok Shop

### 2.1 鉴权模式

- 开发者在 Partner Center 创建 App，获得 `app_key` / `app_secret`。
- 卖家授权走 OAuth 2.0 授权码：`auth_base_url`（如 `https://auth.tiktok-shops.com`）→ 卖家授权 → 回调 `redirect_uri` 携带 `code` → `/api/v2/token/get` 换 `access_token` + `refresh_token`（refresh 有效期长，access 短期需刷新 `/api/v2/token/refresh`）。
- 每次 API 调用需：`app_key`、`timestamp`、按参数字典序拼接后用 `app_secret` 做 HMAC-SHA256 的 `sign`、header `x-tts-access-token`，以及店铺级 `shop_cipher`（授权后经 `/authorization/202309/shops` 获取）。
- 现有 adapter 已实现该签名与 token 刷新（`tiktok/sign.go`、`token` 相关逻辑），配置项：`app_key`、`app_secret`、`auth_base_url`、`api_base_url`、`api_version`、`redirect_uri`、`timeout_sec`。

### 2.2 商品创建必填字段（/product/202309/products）

| 字段 | 说明 |
| --- | --- |
| `title` | 商品标题（各市场长度限制不同，一般 ≤255 字符） |
| `description` | 商品描述（HTML 受限子集）；创建草稿（`save_mode=AS_DRAFT`）时可放宽 |
| `category_id` | 叶子类目 ID，必须来自类目树 |
| `main_images` | 1–9 张，需先经图片上传 API 获得 `uri` |
| `skus` | ≥1 个；每个 SKU 必填 `price.amount`+`price.currency`、`inventory[].quantity`+`warehouse_id`；多规格需 `sales_attributes` |
| `package_weight` | 包裹重量（含单位，跨境物流必填） |
| `is_cod_allowed` 等 | 视市场而定（COD 市场必填） |

- 类目属性：`/product/202309/categories/{category_id}/attributes` 返回属性列表，`is_requried=true` 的属性必须随 `product_attributes` 提交；`/categories/{id}/rules` 返回该类目发品规则（尺码表、资质等）。
- 品牌：`brand_id` 可选，需从品牌接口获取或申请授权品牌。

### 2.3 图片上传

- `POST /product/{version}/images/upload`，multipart 上传，返回 `uri`（图片 URI 用于 main_images/描述图）。
- 限制：单图 ≤5MB（现有 adapter 常量 `maxTikTokListingImageBytes = 5MB`），主图最多 9 张（`maxTikTokMainImages = 9`），格式 JPG/JPEG/PNG，建议 ≥600×600。

### 2.4 限频

- 按 App 维度限频，公开文档口径约为默认 50 QPS/App（不同接口有单独配额，超出返回 429/特定错误码 `36004xxx`）。
- 实务建议：接入统一 provider 限流（现有 `providerlimit`），对 429 做指数退避重试，写操作幂等键防重。

### 2.5 开发者账号申请步骤（供老板申请）

1. 先注册 TikTok Shop 卖家账号（Seller Center，按目标市场：US/UK/东南亚等），需营业执照、法人证件、银行账户，名称需一致，审核约 1–3 个工作日。
2. 访问 https://partner.tiktokshop.com 注册 Partner Center 账号（选择 Developer 类型；自用集成可选「Custom/Self-built developer」，服务商选「App developer」）。
3. 填写公司主体信息并完成开发者资质审核。
4. 在 Partner Center「App & Service → Create App」创建应用：填写应用名称、回调地址（`redirect_uri`，即本系统 `https://<域名>/api/v1/stores/tiktok/callback`）、勾选 API scope（至少 Product、Order、Fulfillment、Seller Authorization）。
5. 审核通过后获得 `app_key` / `app_secret`，填入本系统「设置 → 平台发布配置 → TikTok Shop（platform_tiktok）」。
6. 用卖家账号访问授权链接完成店铺授权，系统换取 token 并拉取 `shop_cipher`。
7. 可先用沙箱（Partner Center Sandbox + API Testing Tool）联调再切生产。

## 3. Shopee（虾皮）

### 3.1 鉴权模式

- 开发者在 Open Platform 创建 App，获得 `partner_id` / `partner_key`。
- 卖家授权：拼接授权链接 `/api/v2/shop/auth_partner`（含 partner_id、timestamp、sign、redirect）→ 卖家登录授权 → 回调携带 `code` + `shop_id` → `/api/v2/auth/token/get` 换 `access_token` + `refresh_token`（access 4 小时，refresh 30 天，用 `/api/v2/auth/access_token/get` 刷新）。
- 请求签名：`sign = HMAC-SHA256(partner_key, base_string)`；公共接口 base_string 为 `partner_id + api_path + timestamp`，店铺级接口再拼 `access_token + shop_id`。所有参数走 query string。
- 现有 adapter 已实现（`shopee/sign.go`、`client.go`），配置项：`partner_id`、`partner_key`、`auth_base_url`、`api_base_url`、`redirect_uri`、`timeout_sec`。

### 3.2 商品创建必填字段（v2.product.add_item）

| 字段 | 说明 |
| --- | --- |
| `item_name` | 商品名（各站点长度限制不同，一般 20–120 字符） |
| `description` | 描述（部分站点最少 25/100 字符） |
| `category_id` | 叶子类目 ID（`v2.product.get_category` 类目树） |
| `original_price` | 价格（无规格时必填；有规格时价格在 model 上） |
| `normal_stock`/`seller_stock` | 库存（无规格时必填） |
| `image.image_id_list` | 1–9 张，需先经 `media_space/upload_image` 获得 `image_id` |
| `logistic_info` | 至少启用一个渠道（`v2.logistics.get_channel_list`），必填 |
| `weight` | 重量（kg），多数站点必填 |
| `dimension` | 包裹长宽高（部分物流渠道必填） |
| `attribute_list` | 类目属性中 `mandatory=true` 的必须提交 |
| `brand.brand_id` | 部分类目强制（无品牌传 0 = NoBrand） |

- 类目属性：`v2.product.get_attribute_tree` 返回属性树；`input_type`（下拉/多选/自由文本/combo）与 `format_type`（带单位/日期）决定提交方式；自定义值 `value_id=0` + `original_value_name`。
- 多规格：先 `add_item` 创建，再 `v2.product.init_tier_variation` 初始化规格层级与 model 价格库存（现有 adapter 已留 `PathProductInitTierVariation`）。

### 3.3 图片上传

- `POST /api/v2/media_space/upload_image`，multipart；`scene=normal`（商品图会被裁方图可用 `scene` 控制）。返回 `image_id`。
- 限制：单图 ≤10MB，JPG/JPEG/PNG；主图最多 9 张；建议 ≥1024×1024。

### 3.4 限频

- 按 partner_id 维度限频；公开口径为默认约 1000 次/分钟（不同 API 分组有单独 QPS 配额，超限返回 `error: request.limit.exceeded`）。
- 实务建议同 TikTok：统一限流 + 退避重试 + 幂等。

### 3.5 开发者账号申请步骤（供老板申请）

1. 先具备 Shopee 卖家账号（本土店需当地主体；中国大陆主体走 CNSC 全球店 China Seller Center，跨境常用）。
2. 访问 https://open.shopee.com 注册 Open Platform 开发者账号（用邮箱注册，绑定公司主体信息完成开发者认证；TW 站点有额外开发者审核）。
3. 控制台「App Management → Create App」创建应用：选择 App 类型（自用 ERP 选 Seller-Own/Private；服务商选 Public），填写回调地址（`https://<域名>/api/v1/stores/shopee/callback`）、申请 API 权限集（Product、MediaSpace、Logistics、Order）。
4. 通过审核后获得 `partner_id` / `partner_key`（先发放 Sandbox 凭证，Go-Live 审核后发放生产凭证），填入本系统「设置 → 平台发布配置 → Shopee（platform_shopee）」。
5. 用卖家账号走授权链接完成店铺授权，系统换取 token。
6. 如需访问敏感数据（买家个人信息等）需单独提交 Sensitive Data 申请。

## 4. 与现有系统的映射与降级口径

- Provider 能力声明：两平台 provider 均已注册 product publish 能力；当 settings 分组（`platform_tiktok` / `platform_shopee`）凭证不完整时，`/api/v1/products/:id/publish-targets` 将该平台能力降级为 `local_draft_only`（本地刊登草稿），配置完整且店铺已授权后才具备真实调用条件。
- payload 校验：adapter 在真实调用前按上表必填字段校验（标题、描述、币种、SKU、主图、类目/物流等），校验失败统一走 HTTP 400 / 业务码 40001 / `execution_validation_failed` 口径。
- 真实调用点：`tiktok/product_publish_api.go`、`shopee/product_publish_api.go`；无凭证环境不发起真实请求。
- 演示数据：`seed:demo:full` 预置 TikTok / Shopee 演示店与 app 配置占位，保证降级刊登全流程可演示。

## 5. 风险与注意事项

- 两平台文档与字段随版本迭代（TikTok 按 `api_version` 202309/202312…），上线前需复核目标版本。
- 类目必填属性是最大的动态校验面：真实接入时必须走类目属性接口做动态校验，不能只靠静态清单。
- 图片必须先上平台图床（uri/image_id），外链不被接受；现有 readiness 检查 `MAIN_IMAGES_NOT_UPLOADED` 口径与此一致。
- 凭证（app_secret / partner_key / token）必须加密存储、脱敏展示、不写日志（现有 settings 加密机制沿用）。
