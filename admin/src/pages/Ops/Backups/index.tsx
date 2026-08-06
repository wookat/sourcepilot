import { StatusTag, TmPageContainer } from '@/components/ui';
import { formatRequestError } from '@/constants/errorMessages';
import {
  createBackup,
  downloadBackup,
  fetchBackups,
  holdBackup,
  retryBackupUpload,
  verifyBackup,
  type BackupJob,
} from '@/services/opsP6';
import { isReadonly } from '@/utils/permission';
import { useModel } from '@umijs/max';
import {
  CloudUploadOutlined,
  DatabaseOutlined,
  DownloadOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import { Alert, Button, Modal, Space, Table, Tag, Tooltip, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import { OpsCheckList } from '../opsChecks';

export function verifyDisabledReason(status?: string): string | undefined {
  if (status === 'completed') return undefined;
  if (status === 'manual_review') {
    return '该备份为待人工复核记录，未生成真实备份文件：需先在环境启用 BACKUP_ENABLED 并通过人工审查，再重新创建备份后才能校验。';
  }
  return '仅已完成（completed）的备份可以校验。';
}

export function uploadStatusTag(row: Pick<BackupJob, 'uploadStatus' | 'uploadError'>) {
  switch (row.uploadStatus) {
    case 'uploaded':
      return <Tag color="green">已上传</Tag>;
    case 'failed':
      return (
        <Tooltip title={row.uploadError || '上传失败'}>
          <Tag color="red">上传失败</Tag>
        </Tooltip>
      );
    case 'skipped':
      return (
        <Tooltip title="未配置对象存储，备份仅保存在本地路径（降级模式）。">
          <Tag>仅本地</Tag>
        </Tooltip>
      );
    default:
      return <Tag>-</Tag>;
  }
}

export default function BackupsPage() {
  const { initialState } = useModel('@@initialState');
  const readonly = isReadonly(initialState?.currentUser?.role);
  const [items, setItems] = useState<BackupJob[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchBackups({ page: 1, pageSize: 50 });
      setItems(res.data?.items ?? []);
    } catch {
      message.error('加载备份记录失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const createDryRun = () => {
    Modal.confirm({
      title: '创建备份记录',
      content: '当前操作会通过后端安全门执行；未启用备份时仅生成待复核记录。',
      onOk: async () => {
        try {
          await createBackup({ reason: 'operator requested from admin', dryRun: false });
          message.success('备份任务已创建');
        } catch (e: unknown) {
          message.error(formatRequestError(e, '创建备份失败'));
        } finally {
          await load();
        }
      },
    });
  };

  return (
    <TmPageContainer
      title="备份管理"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>
            刷新
          </Button>
          <Button
            type="primary"
            icon={<DatabaseOutlined />}
            onClick={createDryRun}
            disabled={readonly}
          >
            创建备份
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Alert
          showIcon
          type="info"
          message="备份产物支持上传 S3 兼容对象存储（配置 BACKUP_S3_* 后自动上传，未配置时仅保存本地）；校验通过的 completed 备份可由管理员下载，本地文件缺失时自动从对象存储取回。"
        />
        <Table<BackupJob>
          rowKey="backupId"
          loading={loading}
          dataSource={items}
          columns={[
            { title: '备份编号', dataIndex: 'backupId', width: 220 },
            { title: '环境', dataIndex: 'environment', width: 120 },
            { title: '类型', dataIndex: 'backupType', width: 160 },
            {
              title: '状态',
              dataIndex: 'status',
              width: 130,
              render: (v) => <StatusTag status={String(v ?? '')} />,
            },
            {
              title: '校验',
              dataIndex: 'verificationStatus',
              width: 130,
              render: (v) => <StatusTag status={String(v ?? '')} />,
            },
            {
              title: '加密',
              dataIndex: 'encrypted',
              width: 100,
              render: (v) => <Tag color={v ? 'green' : 'gold'}>{v ? '已启用' : '未启用'}</Tag>,
            },
            { title: '存储', dataIndex: 'storageProvider', width: 120 },
            {
              title: '上传状态',
              dataIndex: 'uploadStatus',
              width: 120,
              render: (_, row) => uploadStatusTag(row),
            },
            {
              title: '上传目标',
              dataIndex: 'uploadTarget',
              width: 220,
              ellipsis: true,
              render: (v) => (v ? <Tooltip title={String(v)}>{String(v)}</Tooltip> : '-'),
            },
            { title: '大小', dataIndex: 'artifactSize', width: 120, align: 'right' },
            { title: '创建时间', dataIndex: 'createdAt', width: 180 },
            {
              title: '操作',
              width: 300,
              render: (_, row) => (
                <Space>
                  <Tooltip title={verifyDisabledReason(row.status)}>
                    <span style={row.status !== 'completed' ? { cursor: 'not-allowed' } : undefined}>
                      <Button
                        size="small"
                        icon={<SafetyCertificateOutlined />}
                        disabled={row.status !== 'completed' || readonly}
                        onClick={() =>
                          void verifyBackup(row.backupId)
                            .then((res) => {
                              const v = res.data;
                              Modal.info({
                                title: `备份校验结果：${v?.status === 'passed' ? '通过' : '失败'}`,
                                width: 560,
                                content: <OpsCheckList checks={v?.details?.checks} />,
                              });
                            })
                            .catch((e: unknown) =>
                              message.error(formatRequestError(e, '校验备份失败')),
                            )
                            .then(load)
                        }
                      >
                        校验备份
                      </Button>
                    </span>
                  </Tooltip>
                  <Button
                    size="small"
                    icon={<DownloadOutlined />}
                    disabled={
                      row.status !== 'completed' || row.verificationStatus !== 'passed' || readonly
                    }
                    onClick={() =>
                      void downloadBackup(row.backupId)
                        .then(() => message.success('备份文件下载已开始'))
                        .catch((e: unknown) => message.error(formatRequestError(e, '下载备份失败')))
                    }
                  >
                    下载
                  </Button>
                  {row.uploadStatus === 'failed' && (
                    <Button
                      size="small"
                      icon={<CloudUploadOutlined />}
                      disabled={readonly}
                      onClick={() =>
                        void retryBackupUpload(row.backupId)
                          .then(() => message.success('备份已重新上传到对象存储'))
                          .catch((e: unknown) =>
                            message.error(formatRequestError(e, '重试上传失败')),
                          )
                          .then(load)
                      }
                    >
                      重试上传
                    </Button>
                  )}
                  <Button
                    size="small"
                    disabled={readonly}
                    onClick={() =>
                      void holdBackup(row.backupId, 'operator manual hold')
                        .then(() => message.success('已设置保留'))
                        .catch((e: unknown) => message.error(formatRequestError(e, '设置保留失败')))
                        .then(load)
                    }
                  >
                    保留
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Space>
    </TmPageContainer>
  );
}
