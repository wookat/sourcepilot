# 版本升级 SOP（含数据迁移）

适用：Docker 生产部署（`docker-compose.prod.yml` + `scripts/deploy-prod.sh`）从旧版本升级到包含数据库迁移的新版本。已按本 SOP 在开发 VM 上完整演练（旧版本部署 → 造存量数据 → 备份 → 升级 → 验证 → 故意制造迁移中断 → 备份恢复 → 清理重跑成功）。

迁移机制：`MIGRATION_RUN_ON_STARTUP=true`（默认）时 backend 启动即执行 `database.AutoMigrate`（GORM 表结构 + 各 round 数据迁移，多实例经 PostgreSQL advisory lock 串行）。迁移失败 backend `os.Exit(1)` 并被 compose 反复重启；**admin/caddy 依赖 backend healthy，迁移失败期间站点整体不可用**，所以升级必须走「备份 → 预检 → 换镜像 → 验证」的顺序。

## 一、升级前（必做）

```bash
cd trademind

# 1. 全量备份（升级失败唯一可靠的回滚依据）
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U trademind -d trademind -Fc \
  > /var/backups/trademind-pre-upgrade-$(date +%F-%H%M).dump

# 2. 预检 SQL：同租户重复订单号（会中断 R95 之后版本的唯一索引迁移）
docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -U trademind -d trademind -c "
    SELECT tenant_id, order_no, COUNT(*)
    FROM orders WHERE deleted_at IS NULL
    GROUP BY tenant_id, order_no HAVING COUNT(*) > 1;"
# 期望 0 行。有结果先按「三、迁移中断处置」清理，再开始升级。

# 3. 阅读目标版本的发布说明 / PR 描述中「数据库」「影响范围」段落，
#    确认是否有额外迁移前置条件。
```

近期带迁移的版本要点：

| 版本 | 迁移 | 存量影响 |
| --- | --- | --- |
| R95（PR #206） | `orders.order_no` 全局唯一 → 租户内唯一（`idx_orders_tenant_order_no`，随后 drop `idx_orders_order_no`） | 同租户存量重复订单号会中断迁移（预检 SQL 见上）；跨租户重复不受影响 |
| R97（PR #208） | tenant 0 的 `report_currency` 汇率配置复制到所有尚未自行配置的存量租户 | 无中断风险；幂等，可重复执行 |

## 二、升级步骤

```bash
git fetch && git checkout <目标版本 commit 或 tag>
./scripts/deploy-prod.sh --no-pull   # 构建 + 滚动重启 + 自动迁移 + 健康检查 + HTTPS 探活
```

升级后验证：

```bash
# 1. 六个服务全部 healthy
docker compose -f docker-compose.prod.yml ps

# 2. 迁移无报错（应无 database_migrate_failed）
docker compose -f docker-compose.prod.yml logs backend | grep -i migrate

# 3. 关键迁移落地抽查（以 R95/R97 为例）
docker compose -f docker-compose.prod.yml exec -T postgres psql -U trademind -d trademind \
  -c "SELECT indexname FROM pg_indexes WHERE tablename='orders' AND indexname LIKE '%order_no%';" \
  -c "SELECT tenant_id, COUNT(*) FROM settings WHERE group_key='report_currency' GROUP BY tenant_id;"
# 期望：仅 idx_orders_tenant_order_no；每个存量租户各有一组 report_currency 行。

# 4. 业务口径抽查：升级前后各拉一次报表/统计（GET /api/v1/orders/stats/sales 等），
#    金额、订单数、折算基准应逐字段一致。
```

## 三、迁移中断处置（以同租户重复订单号为例）

症状：backend 反复重启，日志出现 `database_migrate_failed`；带预检的版本报
`round95 preflight: orders 表存在同租户重复订单号 …（列出 tenant_id/order_no）`，
更早版本报裸错 `could not create unique index "idx_orders_tenant_order_no" (SQLSTATE 23505)`。
此时**数据未被修改**（建索引失败不影响任何行），但站点整体不可用，处置以「快」为先：

1. **先恢复服务**（二选一）：
   - 数据量小、能立刻清理 → 直接跳到第 2 步清理后重启即可（最快）；
   - 需要时间排查 → 回滚：`git checkout <升级前 commit>` + `./scripts/deploy-prod.sh --no-pull`。
     若新版本迁移已改写数据（本例没有），先 `stop backend` 用 `pg_restore --clean --if-exists`
     恢复升级前备份再启动旧版本。
2. **清理重复订单号**（保留每组最早一条，其余删除；也可与业务确认后改号）：

   ```sql
   WITH dup AS (
     SELECT id, ROW_NUMBER() OVER (
       PARTITION BY tenant_id, order_no ORDER BY created_at, id) rn
     FROM orders WHERE deleted_at IS NULL)
   DELETE FROM orders WHERE id IN (SELECT id FROM dup WHERE rn > 1);
   ```

3. **重跑升级**：`./scripts/deploy-prod.sh --no-pull`，再走「二、升级后验证」。

## 四、回滚路径与已知陷阱

- 常规回滚（迁移未改数据）：`git checkout <旧 commit>` + 重新部署即可，数据卷不动。
- 迁移已改数据：`stop backend` → `pg_restore --clean --if-exists < 升级前备份` → 部署旧版本。
- **陷阱 1（跨租户订单号 + 回滚到 R95 之前）**：新版本上线后各租户可以使用相同订单号；一旦产生跨租户重复，再回滚到 R95 之前的版本会因旧版本启动时重建全局唯一索引 `idx_orders_order_no` 而无法启动。回滚前先查：

  ```sql
  SELECT order_no, COUNT(DISTINCT tenant_id) FROM orders
  WHERE deleted_at IS NULL GROUP BY order_no HAVING COUNT(DISTINCT tenant_id) > 1;
  ```

  有结果则只能回滚到 R95 及之后的版本，或先与业务确认改号。
- **陷阱 2（升级失败 = 全站不可用）**：backend 迁移失败会连带 admin/caddy 起不来（无旧实例兜底），升级窗口按整站停机预期安排，务必先做备份与预检把失败概率压到最低。
- 严禁 `docker compose down -v`（删库删证书）。

## 五、演练记录

- 2026-08-04：旧版本（#204）→ 新版本（含 #206/#208）全流程演练通过：存量多租户/订单/运单/导入任务/汇率配置迁移无损，报表数值逐字段一致；同租户重复订单号中断、备份恢复、清理重跑均按本 SOP 复现并收口。演练发现的迁移预检（round95 preflight）已随同批修复合入。
