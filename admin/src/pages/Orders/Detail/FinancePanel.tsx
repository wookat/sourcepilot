import {
  Alert,
  Button,
  Card,
  Col,
  DatePicker,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Select,
  Skeleton,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs, { type Dayjs } from 'dayjs';
import { useCallback, useEffect, useState } from 'react';
import { EmptyState } from '@/components/ui';
import { formatAmount, tabularNumsStyle } from '@/constants/chartTokens';
import {
  SETTLEMENT_LABEL,
  createOrderExpense,
  createPayment,
  deleteOrderExpense,
  deletePayment,
  fetchOrderFinanceSummary,
  type OrderExpenseRow,
  type OrderFinanceSummary,
  type PaymentRecordRow,
} from '@/services/finance';

function baseText(v: number | undefined, base: string): string {
  return v == null ? '未折算' : `${formatAmount(v)} ${base}`;
}

type PaymentFormValues = {
  amount: number;
  currency?: string;
  feeAmount?: number;
  receivedAt: Dayjs;
  channel?: string;
  remark?: string;
};

type ExpenseFormValues = {
  typeCode: string;
  amount: number;
  currency?: string;
  incurredAt?: Dayjs;
  remark?: string;
};

/** 订单财务面板：回款/费用登记 + 实算毛利 vs 估算毛利 */
export default function FinancePanel({
  orderId,
  writable,
}: {
  orderId: string;
  writable: boolean;
}) {
  const [data, setData] = useState<OrderFinanceSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [payOpen, setPayOpen] = useState(false);
  const [expOpen, setExpOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [payForm] = Form.useForm<PaymentFormValues>();
  const [expForm] = Form.useForm<ExpenseFormValues>();

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    fetchOrderFinanceSummary(orderId)
      .then((res) => setData(res ?? null))
      .catch((e: unknown) => setError(e instanceof Error ? e.message : '加载失败，请稍后重试'))
      .finally(() => setLoading(false));
  }, [orderId]);

  useEffect(() => {
    load();
  }, [load]);

  const submitPayment = useCallback(async () => {
    const values = await payForm.validateFields();
    setSaving(true);
    try {
      await createPayment({
        orderId,
        amount: values.amount,
        currency: values.currency?.trim() || undefined,
        feeAmount: values.feeAmount ?? 0,
        receivedAt: values.receivedAt.format('YYYY-MM-DD'),
        channel: values.channel?.trim() || undefined,
        remark: values.remark?.trim() || undefined,
      });
      message.success('回款登记成功');
      setPayOpen(false);
      payForm.resetFields();
      load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '登记失败，请稍后重试');
    } finally {
      setSaving(false);
    }
  }, [orderId, payForm, load]);

  const submitExpense = useCallback(async () => {
    const values = await expForm.validateFields();
    setSaving(true);
    try {
      await createOrderExpense({
        orderId,
        typeCode: values.typeCode,
        amount: values.amount,
        currency: values.currency?.trim() || undefined,
        incurredAt: values.incurredAt ? values.incurredAt.format('YYYY-MM-DD') : undefined,
        remark: values.remark?.trim() || undefined,
      });
      message.success('费用登记成功');
      setExpOpen(false);
      expForm.resetFields();
      load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '登记失败，请稍后重试');
    } finally {
      setSaving(false);
    }
  }, [orderId, expForm, load]);

  const removePayment = useCallback(
    (id: string) => {
      deletePayment(id)
        .then(() => {
          message.success('已删除回款记录');
          load();
        })
        .catch((e: unknown) => message.error(e instanceof Error ? e.message : '删除失败，请稍后重试'));
    },
    [load],
  );

  const removeExpense = useCallback(
    (id: string) => {
      deleteOrderExpense(id)
        .then(() => {
          message.success('已删除费用记录');
          load();
        })
        .catch((e: unknown) => message.error(e instanceof Error ? e.message : '删除失败，请稍后重试'));
    },
    [load],
  );

  if (loading) {
    return <Skeleton active paragraph={{ rows: 6 }} />;
  }
  if (error) {
    return <Alert type="error" showIcon message="财务信息加载失败" description={error} action={<Button size="small" onClick={load}>重试</Button>} />;
  }
  if (!data) {
    return <EmptyState description="暂无财务信息" />;
  }

  const fin = data.finance;
  const base = data.baseCurrency;
  const settlement = SETTLEMENT_LABEL[fin.settlementStatus] ?? { text: fin.settlementStatus, color: 'default' };

  const payColumns: ColumnsType<PaymentRecordRow> = [
    { title: '回款日期', dataIndex: 'receivedAt', width: 120, render: (v: string) => (v ? dayjs(v).format('YYYY-MM-DD') : '—') },
    { title: '金额', dataIndex: 'amount', width: 130, align: 'right', render: (v: number, r) => <span style={tabularNumsStyle}>{`${formatAmount(v)} ${r.currency}`}</span> },
    { title: '手续费', dataIndex: 'feeAmount', width: 110, align: 'right', render: (v: number) => <span style={tabularNumsStyle}>{formatAmount(v)}</span> },
    { title: '渠道', dataIndex: 'channel', width: 110, render: (v?: string) => v || '—' },
    { title: '来源', dataIndex: 'source', width: 90, render: (v: string) => (v === 'import' ? <Tag>导入</Tag> : <Tag color="blue">手工</Tag>) },
    { title: '备注', dataIndex: 'remark', ellipsis: true, render: (v?: string) => v || '—' },
  ];
  if (writable) {
    payColumns.push({
      title: '操作',
      key: 'action',
      width: 90,
      render: (_, r) => (
        <Popconfirm title="确认删除该回款记录？" onConfirm={() => removePayment(r.id)}>
          <Typography.Link type="danger">删除</Typography.Link>
        </Popconfirm>
      ),
    });
  }

  const expColumns: ColumnsType<OrderExpenseRow> = [
    { title: '费用类型', dataIndex: 'typeLabel', width: 130, render: (v: string, r) => v || r.typeCode },
    { title: '金额', dataIndex: 'amount', width: 130, align: 'right', render: (v: number, r) => <span style={tabularNumsStyle}>{`${formatAmount(v)} ${r.currency}`}</span> },
    { title: '发生日期', dataIndex: 'incurredAt', width: 120, render: (v?: string) => (v ? dayjs(v).format('YYYY-MM-DD') : '—') },
    { title: '备注', dataIndex: 'remark', ellipsis: true, render: (v?: string) => v || '—' },
  ];
  if (writable) {
    expColumns.push({
      title: '操作',
      key: 'action',
      width: 90,
      render: (_, r) => (
        <Popconfirm title="确认删除该费用记录？" onConfirm={() => removeExpense(r.id)}>
          <Typography.Link type="danger">删除</Typography.Link>
        </Popconfirm>
      ),
    });
  }

  return (
    <Row gutter={[16, 16]}>
      <Col span={24}>
        <Card size="small" title="对账概览">
          <Row gutter={[16, 16]}>
            <Col xs={12} sm={8} md={6}>
              <Statistic title={`应收（${fin.currency}）`} value={formatAmount(fin.receivable)} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic title={`已回款（${fin.currency}）`} value={formatAmount(fin.received)} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Statistic title={`回款差异（${fin.currency}）`} value={formatAmount(fin.diffAmount)} />
            </Col>
            <Col xs={12} sm={8} md={6}>
              <Descriptions column={1} size="small">
                <Descriptions.Item label="对账状态">
                  <Tag color={settlement.color}>{settlement.text}</Tag>
                  {fin.largeDiff ? <Tag color="red">差异较大</Tag> : null}
                </Descriptions.Item>
              </Descriptions>
            </Col>
          </Row>
        </Card>
      </Col>
      <Col span={24}>
        <Card size="small" title="实算毛利 vs 估算毛利">
          {fin.missingActualLines > 0 ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message={`有 ${fin.missingActualLines} 条采购明细未登记实付价，实算成本可能不完整`}
            />
          ) : null}
          <Descriptions column={{ xs: 1, sm: 2, md: 3 }} size="small" bordered>
            <Descriptions.Item label="净回款（本位币）">{baseText(fin.receivedBase, base)}</Descriptions.Item>
            <Descriptions.Item label="采购实付（本位币）">{baseText(fin.actualCostBase, base)}</Descriptions.Item>
            <Descriptions.Item label="费用（本位币）">{baseText(fin.expenseBase, base)}</Descriptions.Item>
            <Descriptions.Item label="实算毛利">{baseText(fin.actualProfitBase, base)}</Descriptions.Item>
            <Descriptions.Item label="估算毛利（参考价口径）">{baseText(fin.estimatedProfitBase, base)}</Descriptions.Item>
            <Descriptions.Item label="毛利差异">{baseText(fin.profitDiffBase, base)}</Descriptions.Item>
          </Descriptions>
          <Typography.Paragraph type="secondary" style={{ marginTop: 12, marginBottom: 0 }}>
            实算毛利 = 回款（扣手续费）− 采购实付 − 费用；未配置汇率的币种不折算、不伪造。
          </Typography.Paragraph>
        </Card>
      </Col>
      <Col span={24}>
        <Card
          size="small"
          title="回款记录"
          extra={writable ? <Button type="primary" size="small" onClick={() => setPayOpen(true)}>登记回款</Button> : null}
        >
          <Table<PaymentRecordRow>
            rowKey="id"
            size="small"
            columns={payColumns}
            dataSource={data.payments}
            pagination={false}
            scroll={{ x: 'max-content' }}
            locale={{ emptyText: <EmptyState description="暂无回款记录" /> }}
          />
        </Card>
      </Col>
      <Col span={24}>
        <Card
          size="small"
          title="订单费用"
          extra={writable ? <Button size="small" onClick={() => setExpOpen(true)}>登记费用</Button> : null}
        >
          <Table<OrderExpenseRow>
            rowKey="id"
            size="small"
            columns={expColumns}
            dataSource={data.expenses}
            pagination={false}
            scroll={{ x: 'max-content' }}
            locale={{ emptyText: <EmptyState description="暂无费用记录" /> }}
          />
        </Card>
      </Col>

      <Modal
        title="登记回款"
        open={payOpen}
        onOk={submitPayment}
        confirmLoading={saving}
        onCancel={() => setPayOpen(false)}
        destroyOnHidden
      >
        <Form form={payForm} layout="vertical" initialValues={{ receivedAt: dayjs() }}>
          <Form.Item name="amount" label="回款金额" rules={[{ required: true, message: '请输入回款金额' }]}>
            <InputNumber min={0.01} precision={2} style={{ width: '100%' }} placeholder="平台实际打款金额" />
          </Form.Item>
          <Form.Item name="currency" label="币种" tooltip="留空默认使用订单币种">
            <Input placeholder={fin.currency} maxLength={8} />
          </Form.Item>
          <Form.Item name="feeAmount" label="手续费">
            <InputNumber min={0} precision={2} style={{ width: '100%' }} placeholder="0.00" />
          </Form.Item>
          <Form.Item name="receivedAt" label="回款日期" rules={[{ required: true, message: '请选择回款日期' }]}>
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="channel" label="回款渠道">
            <Input placeholder="如：平台结算 / PayPal / 银行转账" maxLength={64} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} maxLength={512} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="登记订单费用"
        open={expOpen}
        onOk={submitExpense}
        confirmLoading={saving}
        onCancel={() => setExpOpen(false)}
        destroyOnHidden
      >
        <Form form={expForm} layout="vertical">
          <Form.Item name="typeCode" label="费用类型" rules={[{ required: true, message: '请选择费用类型' }]}>
            <Select
              options={data.expenseTypes.map((t) => ({ label: t.label, value: t.code }))}
              placeholder="平台佣金 / 推广费 / 运费 / 其他"
            />
          </Form.Item>
          <Form.Item name="amount" label="费用金额" rules={[{ required: true, message: '请输入费用金额' }]}>
            <InputNumber min={0.01} precision={2} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="currency" label="币种" tooltip="留空默认使用订单币种">
            <Input placeholder={fin.currency} maxLength={8} />
          </Form.Item>
          <Form.Item name="incurredAt" label="发生日期">
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} maxLength={512} />
          </Form.Item>
        </Form>
      </Modal>

    </Row>
  );
}
