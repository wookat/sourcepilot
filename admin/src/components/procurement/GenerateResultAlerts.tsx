import { Alert } from 'antd';
import type { GenerateIssue } from '@/services/procurement';

/** 生成采购单结果的分组提示：blockers、已覆盖（line.covered）、缺参考进价。 */
export default function GenerateResultAlerts({
  blockers,
  warnings,
}: {
  blockers?: GenerateIssue[];
  warnings?: GenerateIssue[];
}) {
  const covered = (warnings || []).filter((w) => w.code === 'line.covered');
  const missingPrice = (warnings || []).filter((w) => w.code !== 'line.covered');
  return (
    <>
      {(blockers || []).length > 0 ? (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          message="部分订单行未能进入采购清单"
          description={
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {(blockers || []).map((b, i) => (
                <li key={i}>
                  {b.message}
                  {b.skuName ? `（${b.skuName}）` : ''}
                </li>
              ))}
            </ul>
          }
        />
      ) : null}
      {covered.length > 0 ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="部分明细已有有效采购单覆盖，未重复生成"
          description={
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {covered.map((w, i) => (
                <li key={i}>
                  {w.message}
                  {w.skuName ? `（${w.skuName}）` : ''}
                </li>
              ))}
            </ul>
          }
        />
      ) : null}
      {missingPrice.length > 0 ? (
        <Alert
          type="warning"
          showIcon
          message="部分明细缺参考进价，采购单金额不含这些行"
          description={
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {missingPrice.map((w, i) => (
                <li key={i}>
                  {w.message}
                  {w.skuName ? `（${w.skuName}）` : ''}
                </li>
              ))}
            </ul>
          }
        />
      ) : null}
    </>
  );
}
