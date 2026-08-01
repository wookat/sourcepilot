import { history } from '@umijs/max';
import { Alert, Button } from 'antd';
import type { GenerateIssue } from '@/services/procurement';

function issueAction(issue: GenerateIssue): { text: string; path: string } | null {
  if (issue.code === 'sku.unmatched' && issue.orderId) {
    return { text: '去规格匹配', path: `/orders/${issue.orderId}?tab=sku` };
  }
  if (issue.code === 'source.missing' && issue.productId) {
    return {
      text: '去绑定主货源',
      path: `/sourcing/product-sources?productId=${issue.productId}&action=bind`,
    };
  }
  if (issue.code === 'mapping.missing' && issue.productId) {
    const sku = issue.localSkuId ? `&skuId=${issue.localSkuId}` : '';
    return {
      text: '去补 SKU 映射',
      path: `/sourcing/product-sources?productId=${issue.productId}&action=mapping${sku}`,
    };
  }
  return null;
}

function IssueList({
  issues,
  onNavigate,
}: {
  issues: GenerateIssue[];
  onNavigate?: () => void;
}) {
  return (
    <ul style={{ margin: 0, paddingLeft: 18 }}>
      {issues.map((b, i) => {
        const action = issueAction(b);
        return (
          <li key={i}>
            {b.message}
            {b.skuName ? `（${b.skuName}）` : ''}
            {action ? (
              <Button
                type="link"
                size="small"
                style={{ paddingInline: 4 }}
                onClick={() => {
                  onNavigate?.();
                  history.push(action.path);
                }}
              >
                {action.text}
              </Button>
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}

/** 生成采购单结果的分组提示：blockers、已覆盖（line.covered）、缺参考进价。 */
export default function GenerateResultAlerts({
  blockers,
  warnings,
  onNavigate,
}: {
  blockers?: GenerateIssue[];
  warnings?: GenerateIssue[];
  /** 点击某条阻塞项的直达链接跳转前触发（用于关闭承载弹窗）。 */
  onNavigate?: () => void;
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
          description={<IssueList issues={blockers || []} onNavigate={onNavigate} />}
        />
      ) : null}
      {covered.length > 0 ? (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="部分明细已有有效采购单覆盖，未重复生成"
          description={<IssueList issues={covered} onNavigate={onNavigate} />}
        />
      ) : null}
      {missingPrice.length > 0 ? (
        <Alert
          type="warning"
          showIcon
          message="部分明细缺参考进价，采购单金额不含这些行"
          description={<IssueList issues={missingPrice} onNavigate={onNavigate} />}
        />
      ) : null}
    </>
  );
}
