# R183 线2：竞品复评 v11（店小秘 / 马帮 / AutoDS）

- 日期：2026-08-08
- 角色：product-researcher + qa-engineer
- 产出：`docs/COMPETITIVE_BENCHMARK_R183.md`

## 做了什么

1. **#360–#366 合并状态权威核实**（GitHub PR 状态）：#360（W1）、#364（R181 线2）已合入 main；#361/#362/#363/#365/#366 仍 OPEN。
2. **实测口径**：因 W2/W3/R182 未合入，按叠加分支实测（origin/main + #365 分支，含 W1–W3 全量与 advisory lock），报告中 main 口径与叠加口径分列标注。
3. **关键能力抽验（Docker 全栈，无回退确认）**：
   - 三层闸门逐层独立拒绝（env 关→写工具不注册；租户关→调用被拒；无 write:ops→不可见）。
   - 写六动作全链路：W1 打标 dry_run→execute→重放幂等→参数漂移拒绝；W2 exceptions_mark/mark-placed/fill-logistics 状态机走通（DB 复核）；W3 mark-paid 金额前提三连（未配置拒绝/差一分拒绝/精确一致通过）。
   - 治理：operator 创建写 token 403、写 token 对 operator 不可见、跨租户 404 口径、审计逐行落库（mode/params_summary/confirm_hash/amount）。
   - 只读面零回退：四只读工具正常、客户名脱敏、healthz 全绿。
4. **竞品 2026-08 复查**：本轮相对 R178（同日）零增量；AutoDS MCP 常态叙事、店小秘/马帮仍零 MCP。
5. **矩阵结论**：超越 5 / 达到 11 / 落后 0；「AI 对话式写操作」叠加口径由落后转**超越**（写面窄但治理深度独有），main 口径持平。

## 遗留与建议

- 5 个 OPEN PR（#361/#362/#363/#365/#366）待按序合入，合入后「超越」位次才在 main 口径成立。
- 下阶段：①PR 积压清理（0.5 轮）②「有治理的对话式运营」叙事包（0.5 轮）③复评回归常态节奏（下次默认 R195 前后）④写面扩展保持克制（触发式）。

## 收尾

- 测试 token 全部吊销、临时 settings 复位、demo 数据 clean+verify 零残留；证据作会话附件不入库；Actions CI 不作依据。
