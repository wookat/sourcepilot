# 版本升级 SOP（含数据迁移）

适用：Docker 生产部署（`docker-compose.prod.yml` + `scripts/deploy-prod.sh`）从旧版本升级到包含数据库迁移的新版本。已按本 SOP 在开发 VM 上完整演练（旧版本部署 → 造存量数据 → 备份 → 升级 → 验证 → 故意制造迁移中断 → 备份恢复 → 清理重跑成功）。

迁移机制：`MIGRATION_RUN_ON_STARTUP=true`（默认）时 backend 启动即执行 `database.AutoMigrate`（GORM 表结构 + 各 round 数据迁移，多实例经 PostgreSQL advisory lock 串行）。迁移失败 backend `os.Exit(1)` 并被 compose 反复重启；**admin/caddy 依赖 backend healthy，迁移失败期间站点整体不可用**，所以升级必须走「备份 → 预检 → 换镜像 → 验证」的顺序。

## 一、升级前（必做）

```bash
cd trademind

# 一键方式：备份 + 预检一步完成（备份目录默认 /var/backups，可用 BACKUP_DIR=... 覆盖）
./scripts/deploy-prod.sh --pre-upgrade-check

# 或手动逐步执行：
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
| R102/R103（PR #216 及后续） | `collect_rules`、`collect_browser_profiles` 新增 `tenant_id` 列（默认 0，索引） | 无中断风险；但**存量行全部落在 tenant 0（平台租户）**，业务租户升级后将看不到自己此前创建的采集规则 / 浏览器 profile，需按下方「R102 采集规则租户归属回填」处理 |

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

### R102 采集规则租户归属回填（升级到含 PR #216 的版本后）

R102 起 `collect_rules`、`collect_browser_profiles` 按租户隔离，存量行迁移后默认归属 tenant 0（平台租户），业务租户在页面上将看不到自己升级前创建的规则 / profile。升级后按需回填：

```bash
# 预检：确认有多少存量行落在 tenant 0，以及部署内有哪些业务租户
docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -U trademind -d trademind \
  -c "SELECT 'collect_rules' AS tbl, tenant_id, COUNT(*) FROM collect_rules GROUP BY tenant_id
      UNION ALL
      SELECT 'collect_browser_profiles', tenant_id, COUNT(*) FROM collect_browser_profiles GROUP BY tenant_id;" \
  -c "SELECT id, name, status FROM tenants WHERE deleted_at IS NULL ORDER BY id;"
```

- **单业务租户部署**（tenant 0 仅作平台桶、日常采集都在同一个业务租户）：整体回填到该租户，例如租户 id 为 1：

  ```sql
  UPDATE collect_rules SET tenant_id = 1 WHERE tenant_id = 0;
  UPDATE collect_browser_profiles SET tenant_id = 1 WHERE tenant_id = 0;
  ```

- **多业务租户部署**：无法从数据本身推断归属，需按创建人 / 业务线逐条确认后分配（`UPDATE ... SET tenant_id = <目标租户> WHERE id IN (...)`）；确认前保留在 tenant 0 只影响业务租户的可见性，不影响平台租户使用，也不会跨租户泄露。
- 平台自用的规则 / profile 保留 `tenant_id = 0` 即可。
- 回填幂等、可分批执行；执行前建议按「一、升级前」做好备份。

### 业务租户 `/ops/*` 关闭后的替代口径（R102）

R102 起 `/api/v1/ops/*` 中的备份 / 恢复 / 发布 / 容灾接口收紧为平台租户（tenant 0 admin）专属：这些操作作用于**整个部署**（如全库备份可导出所有租户数据），不适合暴露给单个业务租户。替代口径：

- **备份 / 恢复**：由平台管理员按本 SOP 统一执行部署级备份；业务租户如需自己数据的导出，走各业务列表的导出能力或由平台管理员代为提取，暂不提供租户级自助备份。
- **发布 / 容灾演练**：仅平台管理员操作，业务租户无需感知。
- 业务租户仍可使用的运维页：后台任务监控、任务中心、可观测性、平台运行状态（均按租户隔离数据）。
- R103 起业务租户 admin 的侧边栏不再展示备份 / 恢复 / 发布 / 容灾入口；直接访问对应 `/ops/*` API 返回 403。

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
- **陷阱 2（升级失败 = 全站不可用）**：backend 迁移失败会连带 admin/caddy 起不来（无旧实例兜底），升级窗口按整站停机预期安排，务必先做备份与预检（`--pre-upgrade-check`）把失败概率压到最低。现有友好提示兜底：admin 容器不可达时 Caddy 返回「系统升级维护中」维护页；backend 不可用时 nginx 对 `/api` 返回统一 JSON（503「系统升级维护中，请稍后重试」），不再是裸 502。真正零停机（蓝绿/迁移与启动解耦）需要双实例编排，不在当前单机 compose 架构范围内。
- 严禁 `docker compose down -v`（删库删证书）。

## 五、演练记录

- 2026-08-04：旧版本（#204）→ 新版本（含 #206/#208）全流程演练通过：存量多租户/订单/运单/导入任务/汇率配置迁移无损，报表数值逐字段一致；同租户重复订单号中断、备份恢复、清理重跑均按本 SOP 复现并收口。演练发现的迁移预检（round95 preflight）已随同批修复合入。
