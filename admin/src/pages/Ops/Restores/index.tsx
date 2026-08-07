import { StatusTag, TmPageContainer } from '@/components/ui';
import { formatRequestError, formatUserErrorMessage } from '@/constants/errorMessages';
import { createRestore, fetchRestores, verifyRestore, type RestoreJob } from '@/services/opsP6';
import { formatDateTime } from '@/utils/formatTime';
import { ReloadOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { Alert, Button, Form, Input, Modal, Space, Switch, Table, Tooltip, Typography, message } from 'antd';
import { useCallback, useEffect, useRef, useState } from 'react';
import { OpsCheckList } from '../opsChecks';

export default function RestoresPage() {
  const [items, setItems] = useState<RestoreJob[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetchRestores({ page: 1, pageSize: 50 });
      setItems(res.data?.items ?? []);
    } catch {
      message.error('加载恢复记录失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <TmPageContainer
      title="恢复验证"
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={() => void load()} loading={loading}>
            刷新
          </Button>
          <Button type="primary" icon={<SafetyCertificateOutlined />} onClick={() => setOpen(true)}>
            创建恢复验证
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Alert
          showIcon
          type="warning"
          message="恢复默认只允许隔离环境（本地/开发限定）；验证仅真实执行备份文件完整性与 pg_restore 结构校验，其余检查项标注暂未实现；真实生产恢复和 RPO/RTO 验证保持待接入。"
        />
        <Table<RestoreJob>
          rowKey="restoreId"
          loading={loading}
          dataSource={items}
          columns={[
            { title: '恢复编号', dataIndex: 'restoreId', width: 220 },
            { title: '备份编号', dataIndex: 'backupId', width: 220 },
            { title: '目标环境', dataIndex: 'targetEnvironment', width: 140 },
            { title: '状态', dataIndex: 'status', width: 120, render: (v) => <StatusTag status={String(v ?? '')} /> },
            { title: '安全门', dataIndex: 'safetyGateStatus', width: 120, render: (v) => <StatusTag status={String(v ?? '')} /> },
            {
              title: '失败原因',
              dataIndex: 'errorSummary',
              width: 260,
              render: (v?: string) =>
                v ? (
                  <Tooltip title={formatUserErrorMessage(v, v)}>
                    <Typography.Text type="danger" ellipsis style={{ maxWidth: 240 }}>
                      {formatUserErrorMessage(v, v)}
                    </Typography.Text>
                  </Tooltip>
                ) : (
                  '—'
                ),
            },
            { title: '完整性', dataIndex: 'validationStatus', width: 120, render: (v) => (v ? <StatusTag status={String(v)} /> : '—') },
            {
              title: '创建时间',
              dataIndex: 'createdAt',
              width: 180,
              render: (v?: string) => formatDateTime(v),
            },
            {
              title: '操作',
              width: 140,
              render: (_, row) => (
                <Button
                  size="small"
                  onClick={() =>
                    void verifyRestore(row.restoreId)
                      .then((res) => {
                        const v = res.data;
                        Modal.info({
                          title: `恢复验证结果：${v?.status === 'passed' ? '通过（仅真实检查项）' : '失败'}`,
                          width: 560,
                          content: <OpsCheckList checks={v?.details?.checks} />,
                        });
                      })
                      .catch((e: unknown) => message.error(formatRequestError(e, '验证失败')))
                      .then(load)
                  }
                >
                  验证结果
                </Button>
              ),
            },
          ]}
        />
      </Space>
      <Modal
        title="创建恢复验证"
        open={open}
        confirmLoading={submitting}
        onCancel={() => setOpen(false)}
        onOk={async () => {
          if (submittingRef.current) return;
          submittingRef.current = true;
          setSubmitting(true);
          let values;
          try {
            values = await form.validateFields();
          } catch {
            submittingRef.current = false;
            setSubmitting(false);
            return;
          }
          try {
            await createRestore(values);
            message.success('恢复验证已创建');
            setOpen(false);
            form.resetFields();
          } catch (e: unknown) {
            message.error(formatRequestError(e, '创建恢复验证失败'));
          } finally {
            submittingRef.current = false;
            setSubmitting(false);
            await load();
          }
        }}
      >
        <Form form={form} layout="vertical" initialValues={{ targetEnvironment: 'isolated', targetIsIsolated: true }}>
          <Form.Item name="backupId" label="备份编号" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="targetEnvironment" label="目标环境" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="targetDatabaseName" label="目标数据库摘要" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="targetIsIsolated" label="已确认隔离环境" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="operatorReauthenticated" label="已完成二次确认" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="highRiskConfirmed" label="已确认高风险操作" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
