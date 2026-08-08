# R179 线2：R178 演练 P2 收口（存量明文敏感项读路径脱敏 + 惰性加密收编）

- 轮次：R179 线2（fullstack-engineer）
- 日期：2026-08-08
- 范围：P2-R178-1 收口 + R176–R178 合入面巡检
- 分支基线：叠加 PR #355 分支（`devin/1786174620-r177-line1`，演练时仍未合入 main），PR → main，不自行 merge
- Actions CI 不作依据，全部结论来自本地门禁与 Docker 实测；证据外置不入库

## 1. P2-R178-1：存量明文敏感项收口

### 问题（R178 演练登记）

注册表命中的敏感 key（如 `platform_tiktok/app_secret`）若为历史版本明文落库（`is_encrypted=false`），GET 读回显不脱敏，直到首次改写才收编加密；`sensitive_registry.go` 注释"masked on read, regardless of the client-supplied isEncrypted"对存量明文行不成立。

### 修复（`backend/internal/modules/settings/service.go`）

- 读路径（`Service.List`）不再仅依赖 `is_encrypted` 标志：对 `is_encrypted=false` 的行先查注册表 `IsSensitiveKey`，命中即脱敏回显并回显 `isEncrypted:true`；表外明文行保持兼容口径不变。
- **惰性加密评估：可行，已实现**（`Service.adoptLegacyPlaintext`）。安全性依据：
  1. UPDATE 谓词带乐观并发保护 `WHERE id=? AND is_encrypted=false AND item_value=<读到的明文>`——并发 PUT（写路径本就强制加密）先落库时该 UPDATE 命中 0 行，不会覆盖新值；
  2. 尽力而为：加密失败或 UPDATE 失败均不影响 GET 可用性，本次响应仍脱敏；
  3. 未配置 encrypter（无 `APP_MASTER_KEY`）时不改写，仅读时脱敏，行为可预期。
- 注册表注释修正：明确「写路径强制加密；存量明文行读时脱敏并在有 encrypter 时惰性收编加密」。

### 回归（先红后绿）

- 单测（settings 包，自注册测试 key 自包含）：`TestListMasksLegacyPlaintextSensitiveRow`、`TestListLazilyEncryptsLegacyPlaintextSensitiveRow`、`TestListMasksLegacyPlaintextWithoutEncrypter`、`TestListLeavesNonRegistryPlaintextAlone`（`r179_legacy_plaintext_test.go`）。
- 生产路由级（permmatrix）：`TestSettingsLegacyPlaintextSensitiveMaskedAndAdoptedOnRead`（`r179_legacy_plaintext_settings_test.go`）——种子明文行 → GET 不泄露明文、`isEncrypted:true`、DB 落库变密文。
- 红验证：stash 掉 service.go 修复后该生产路由测试失败（明文回显），恢复后通过。

## 2. Docker 双租户实测（升级存量明文场景构造）

`docker-compose.full.yml` 全栈（postgres/redis/collector/backend/admin），注册两租户（tenant 2/3），直接 SQL 注入两行存量明文敏感项（`platform_tiktok/app_secret`，`is_encrypted=false`，模拟注册表引入前版本落库）：

- GET /settings：双租户各自脱敏回显 `r17****cret`、`isEncrypted:true`，无明文泄露，无跨租户泄露；
- 读后 DB：两行均 `is_encrypted=t`、item_value 为密文，明文不再存在于落库值；
- 二次读：解密成功后按明文脱敏回显（证明惰性收编后的密文可正常解密），updatedAt 变更一次后稳定。

证据（响应 JSON、DB 前后快照、脚本输出）外置于演练机 `/tmp/r179-evidence/`，作附件不 commit。

## 3. R176–R178 合入面巡检（抽验）

- console 告警：R175..HEAD admin 增量 diff 无新增 `console.log/warn/error`。
- 裸英文/裸枚举：admin 增量中的英文字符串仅为 API reason 参数（`'operator requested from admin'`、`'operator rollback'`，非 UI 文案）与测试 describe 名，无新增用户可见裸英文/裸枚举。
- 40303 一致性：`CodeStorePermissionDenied=40303` 仍集中于 `response/codes.go` + `adminperm/deny.go` 单点；增量新增引用均走 `requireCode40303` 测试断言与集中 deny 路径，未发现散写。

## 4. 门禁（本地，全绿）

- backend：`go vet ./...`、`gofmt -l .`（空）、`APP_ENV=test go test ./...`（103 包 ok，含 permmatrix 全量）、`pnpm test:backend:integration`
- 前端/采集/契约：`pnpm check:ui-copy --strict`、`pnpm test:frontend`、`pnpm test:collector`、`pnpm test:contracts`、`pnpm build:admin`、`pnpm build:collector` 全部通过
- `pnpm check:dev`：仅报缺 `.env`（环境检查项，非代码问题）

## 登记

- P2-R178-1：**关闭**（读时脱敏 + 惰性收编加密 + 注释修正 + 回归 + Docker 实测）。
- P2-R178-2（`pnpm audit --prod` 构建工具链告警）：不在本轮范围，继续跟踪。
