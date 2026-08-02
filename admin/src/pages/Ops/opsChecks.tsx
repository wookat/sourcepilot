import type { OpsCheck } from '@/services/opsP6';
import { List, Tag } from 'antd';
import type { ReactNode } from 'react';

const CHECK_LABELS: Record<string, string> = {
  checksum: '备份文件校验和（SHA-256）',
  pg_restore_list: 'pg_restore --list 结构校验',
  manifest: '清单（manifest）完整性',
  encryption: '备份加密',
  backup_file_integrity: '备份文件完整性（SHA-256）',
  migration_version: '迁移版本核对',
  tenant_isolation: '租户隔离核对',
  rbac: '角色权限核对',
  audit_chain: '审计链核对',
  object_inventory: '对象清单核对',
  secret_ciphertext: '密钥密文核对',
  rpo_measurement: 'RPO 实测',
  rto_measurement: 'RTO 实测',
  application_failover: '应用切换演练',
};

const STATUS_META: Record<OpsCheck['status'], { label: string; color: string }> = {
  passed: { label: '通过', color: 'green' },
  failed: { label: '失败', color: 'red' },
  skipped: { label: '未启用（跳过）', color: 'gold' },
  not_implemented: { label: '暂未实现', color: 'default' },
};

export function checkLabel(key: string): string {
  return CHECK_LABELS[key] ?? key;
}

export function checkStatusTag(status: OpsCheck['status']): ReactNode {
  const meta = STATUS_META[status] ?? { label: status, color: 'default' };
  return <Tag color={meta.color}>{meta.label}</Tag>;
}

/** 结构化中文展示一组校验/演练检查项。 */
export function OpsCheckList({ checks }: { checks?: OpsCheck[] }) {
  if (!checks?.length) {
    return <span>无检查项明细</span>;
  }
  return (
    <List
      size="small"
      dataSource={checks}
      renderItem={(item) => (
        <List.Item>
          <List.Item.Meta title={checkLabel(item.key)} description={item.message || undefined} />
          {checkStatusTag(item.status)}
        </List.Item>
      )}
    />
  );
}
