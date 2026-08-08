# R172 线2：MCP 写白名单 D1–D4 决策材料终版（三视角辩论）

- 轮次：R172 线2（tech-lead 主持；同会话内 tech-lead / security-engineer / product-manager 三视角结构化辩论）
- 日期：2026-08-08
- 交付物：`docs/design/MCP_WRITE_WHITELIST_DECISION_BRIEF.md`（决策一页纸 + 完整辩论记录）
- 性质：纯文档，不含任何实现代码

## 做了什么

1. 按 CHARTER「重大方向多角色辩论」要求，围绕 #326 设计稿的四个决策点（D1 五工具白名单含 mark-paid、D2 独立 `write:ops` scope、D3 三层默认关闭闸门+dry-run→确认token→execute、D4 fail-closed 审计限额/operator 管理权/过期策略）进行三视角独立陈述→交叉质疑→收敛。
2. 每个决策点给出：推荐选项、反对意见及反驳、AutoDS 实际写范围对照、采纳时的工作量（轮数）与测试面、不采纳的机会成本。
3. 产出老板勾选式决策一页纸（每 D 一行推荐+理由+勾选框）。

## 结论（摘要）

- **四项全部收敛为一致推荐采纳**；依赖关系明确：D3/D4 是 D1 成立前提、D2 是 D4 载体，建议整体采纳；唯一可独立降级项为 D1 的 W5（降级 W1–W4 不影响其余）。
- W5 mark-paid 进 P0 有条件放行：金额上限租户必填 fail-closed、审单闸门优先、金额进审计摘要、dry_run 预览四要素。
- 唯一真分歧（operator 是否可管写 token）收敛为：P0 仅 admin，operator 只读流水视图；「operator 自助 + 授权联动失效」列 P1 研究项（避免复刻 R165 P1-6 缺口模式）。
- 工作量口径统一为 **2.5 轮**（P0 实现 2 + 安全验收 0.5，含验收的保守数；R170 的 1–1.5 轮为仅后端口径）。
- 辩论新增 5 项实现要求（dry_run summary、四要素预览、scope 组合默认项、Redis 故障 fail-closed、限额校准复盘），登记于 brief §5-3。

## 论据数据来源

- `docs/SECURITY_AUDIT_R165.md`：六处 view-only P1（「可见当可操作」模式）与 `EnsureStoreOperable` 收敛、审计 fail-closed 实测——支撑「LLM 参数幻觉为第一威胁」「治理面不下放」论点。
- `docs/progress/R169-line2.md`：token 治理季度复查全 PASS——支撑 D2 独立 scope 与 D4 过期策略基建就绪判断。
- `docs/COMPETITIVE_BENCHMARK_R170.md`：AutoDS 写操作量产教育期（08-05 多店铺教程）、店小秘/马帮零 MCP——支撑时间窗口与差异化叙事。

## 备注

- 纯文档轮，未改动任何代码/配置，项目代码检查命令不适用。
- company-os 侧说明：仓库无 `roles/security-engineer`，安全视角按最接近的 `roles/qa/code-reviewer.md`（安全评审职责）+ CHARTER §8 执行，已在 brief 头部诚实标注。
- PR → main，不自行 merge（老板决策材料，按任务要求留待勾选）。
