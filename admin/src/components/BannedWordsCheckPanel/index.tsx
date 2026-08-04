import { EmptyState, MetricCard, OperationToolbar, SectionCard } from '@/components/ui';
import {
  checkProductBannedWords,
  type BannedWordHit,
  type BannedWordScanField,
  type BannedWordScanResult,
} from '@/services/bannedWords';
import { SafetyOutlined } from '@ant-design/icons';
import { Alert, Button, Space, Spin, Tag, Typography } from 'antd';
import { useCallback, useMemo, useState } from 'react';

/** 将命中位置切分为文本片段并高亮（位置为 Unicode 码点偏移，与后端一致）。 */
export function highlightSegments(
  text: string,
  ranges: { start: number; end: number; level: string }[],
): { text: string; level?: string }[] {
  const chars = Array.from(text);
  const sorted = [...ranges]
    .filter((r) => r.start >= 0 && r.end > r.start && r.start < chars.length)
    .sort((a, b) => a.start - b.start || b.end - a.end);
  const out: { text: string; level?: string }[] = [];
  let cursor = 0;
  for (const r of sorted) {
    const start = Math.max(r.start, cursor);
    const end = Math.min(r.end, chars.length);
    if (start >= end) continue;
    if (start > cursor) out.push({ text: chars.slice(cursor, start).join('') });
    out.push({ text: chars.slice(start, end).join(''), level: r.level });
    cursor = end;
  }
  if (cursor < chars.length) out.push({ text: chars.slice(cursor).join('') });
  return out;
}

function FieldHighlight({ field, hits }: { field: BannedWordScanField; hits: BannedWordHit[] }) {
  const ranges = hits.flatMap((h) =>
    h.positions.map((p) => ({ start: p.start, end: p.end, level: h.level })),
  );
  const segments = useMemo(
    () => highlightSegments(field.text, ranges),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [field.text, JSON.stringify(ranges)],
  );
  return (
    <div>
      <Typography.Text strong>{field.label}</Typography.Text>
      <div
        style={{
          marginTop: 4,
          padding: '8px 12px',
          background: 'rgba(0,0,0,0.02)',
          borderRadius: 6,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
          maxHeight: 200,
          overflowY: 'auto',
        }}
      >
        {segments.map((seg, i) =>
          seg.level ? (
            <mark
              key={i}
              style={{
                background: seg.level === 'forbidden' ? '#ffccc7' : '#ffe7ba',
                padding: '0 1px',
                borderRadius: 2,
              }}
            >
              {seg.text}
            </mark>
          ) : (
            <span key={i}>{seg.text}</span>
          ),
        )}
      </div>
    </div>
  );
}

export default function BannedWordsCheckPanel({ productId }: { productId: string }) {
  const [result, setResult] = useState<BannedWordScanResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const run = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      setResult(await checkProductBannedWords(productId));
    } catch (e) {
      setResult(null);
      setError((e as Error).message || '合规检测失败');
    } finally {
      setLoading(false);
    }
  }, [productId]);

  const fieldsWithHits = useMemo(() => {
    if (!result?.fields) return [];
    return result.fields
      .map((f) => ({ field: f, hits: (result.hits || []).filter((h) => h.field === f.field) }))
      .filter((x) => x.hits.length > 0);
  }, [result]);

  return (
    <SectionCard
      title="合规检测（违禁词）"
      description="扫描标题、卖点与详情文案中的违禁词。禁止级命中会在发布检查中阻断刊登，警告级仅提示；词库可在「设置 → 违禁词库」维护。"
      id="banned-words-check"
      className="product-draft-banned-words"
      headerExtra={
        <OperationToolbar>
          <Button type="primary" icon={<SafetyOutlined />} loading={loading} onClick={() => void run()}>
            开始合规检测
          </Button>
        </OperationToolbar>
      }
    >
      {error ? (
        <Alert
          type="error"
          showIcon
          message="合规检测失败"
          description={error}
          action={
            <Button size="small" onClick={() => void run()}>
              重试
            </Button>
          }
        />
      ) : loading && !result ? (
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <Spin />
          <Typography.Text type="secondary">正在扫描违禁词。</Typography.Text>
        </div>
      ) : !result ? (
        <EmptyState
          title="尚未执行合规检测"
          description="点击「开始合规检测」扫描当前草稿的标题、卖点与详情文案。"
        />
      ) : (
        <Space direction="vertical" size="large" style={{ width: '100%' }}>
          <div
            style={{
              display: 'grid',
              gap: 12,
              gridTemplateColumns: 'repeat(auto-fit, minmax(160px, 1fr))',
            }}
            aria-label="合规检测摘要"
          >
            <MetricCard
              title="检测结果"
              value={
                result.status === 'blocked' ? (
                  <Tag color="red">禁止级命中</Tag>
                ) : result.status === 'warning' ? (
                  <Tag color="orange">警告级命中</Tag>
                ) : (
                  <Tag color="green">通过</Tag>
                )
              }
              description={result.statusLabel || ''}
              intent={
                result.status === 'blocked'
                  ? 'danger'
                  : result.status === 'warning'
                    ? 'warning'
                    : 'success'
              }
            />
            <MetricCard
              title="禁止级命中"
              value={result.forbiddenCount}
              description={result.forbiddenCount > 0 ? '将阻断刊登，需修改后重试。' : '无阻断命中。'}
              intent={result.forbiddenCount > 0 ? 'danger' : 'default'}
            />
            <MetricCard
              title="警告级命中"
              value={result.warningCount}
              description={result.warningCount > 0 ? '不阻断刊登，建议人工确认。' : '无警告命中。'}
              intent={result.warningCount > 0 ? 'warning' : 'default'}
            />
          </div>
          {result.hits.length === 0 ? (
            <Alert type="success" showIcon message="未检出违禁词" description="当前草稿文案在已启用的词库范围内未发现违禁词。" />
          ) : (
            <>
              <div>
                <Typography.Text strong>命中明细</Typography.Text>
                <ul style={{ margin: '8px 0 0', paddingLeft: 20 }}>
                  {result.hits.map((h, i) => (
                    <li key={`${h.field}-${h.word}-${i}`} style={{ marginBottom: 4 }}>
                      {h.level === 'forbidden' ? <Tag color="red">禁止</Tag> : <Tag color="orange">警告</Tag>}
                      <Typography.Text strong>「{h.word}」</Typography.Text>
                      <Typography.Text type="secondary">
                        {h.fieldLabel} · {h.categoryLabel} · {h.positions.length} 处
                      </Typography.Text>
                      {h.suggestion ? (
                        <div>
                          <Typography.Text type="secondary">建议：{h.suggestion}</Typography.Text>
                        </div>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </div>
              {fieldsWithHits.length > 0 ? (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  {fieldsWithHits.map(({ field, hits }) => (
                    <FieldHighlight key={field.field} field={field} hits={hits} />
                  ))}
                </Space>
              ) : null}
            </>
          )}
        </Space>
      )}
    </SectionCard>
  );
}
