import { UploadOutlined } from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  DatePicker,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import dayjs, { type Dayjs } from 'dayjs';
import { Link, history, useModel } from '@umijs/max';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { EmptyState, TmPageContainer } from '@/components/ui';
import { formatAmount, tabularNumsStyle } from '@/constants/chartTokens';
import { canWriteOrders } from '@/utils/orderPerm';
import { appendSourceToUrl } from '@/utils/urlState';
import {
  SETTLEMENT_LABEL,
  createPayment,
  createShopExpense,
  deletePayment,
  deleteShopExpense,
  queryExpenseTypes,
  queryPayments,
  queryShopExpenses,
  type ExpenseType,
  type PaymentRecordRow,
  type SettlementStatus,
  type ShopExpenseRow,
} from '@/services/finance';
import { queryOrders } from '@/services/orders';
import { queryShops, type ShopListRow } from '@/services/shops';

const STATUS_OPTIONS = [
  { label: '全部状态', value: '' },
  { label: '未回款', value: 'unpaid' },
  { label: '少款', value: 'short' },
  { label: '多款', value: 'over' },
  { label: '已结清', value: 'settled' },
];

type PaymentFormValues = {
  orderNo: string;
  amount: number;
  currency?: string;
  feeAmount?: number;
  receivedAt: Dayjs;
  channel?: string;
  remark?: string;
};

function PaymentsTab({ writable }: { writable: boolean }) {
  const [rows, setRows] = useState<PaymentRecordRow[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [status, setStatus] = useState<'' | SettlementStatus>('');
  const [shopId, setShopId] = useState<string | undefined>(undefined);
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<PaymentFormValues>();

  useEffect(() => {
    queryShops({ page: 1, pageSize: 100 })
      .then((res) => setShops(res.list ?? []))
      .catch(() => setShops([]));
  }, []);

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    queryPayments({ page, pageSize, status, shopId })
      .then((res) => {
        setRows(res.items ?? []);
        setTotal(res.total ?? 0);
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : '加载失败，请稍后重试'))
      .finally(() => setLoading(false));
  }, [page, pageSize, status, shopId]);

  useEffect(() => {
    load();
  }, [load]);

  const submit = useCallback(async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const found = await queryOrders({ orderNo: values.orderNo.trim(), page: 1, pageSize: 1 });
      const order = (found.list ?? [])[0];
      if (!order || order.orderNo !== values.orderNo.trim()) {
        message.error('未找到该订单号对应的订单');
        return;
      }
      await createPayment({
        orderId: order.id,
        amount: values.amount,
        currency: values.currency?.trim() || undefined,
        feeAmount: values.feeAmount ?? 0,
        receivedAt: values.receivedAt.format('YYYY-MM-DD'),
        channel: values.channel?.trim() || undefined,
        remark: values.remark?.trim() || undefined,
      });
      message.success('回款登记成功');
      setOpen(false);
      form.resetFields();
      load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '登记失败，请稍后重试');
    } finally {
      setSaving(false);
    }
  }, [form, load]);

  const remove = useCallback(
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

  const columns = useMemo<ColumnsType<PaymentRecordRow>>(() => {
    const cols: ColumnsType<PaymentRecordRow> = [
      {
        title: '订单号',
        dataIndex: 'orderNo',
        width: 180,
        render: (v: string, r) => (
          <Link to={appendSourceToUrl(`/orders/${r.orderId}`, 'finance-payments')}>{v}</Link>
        ),
      },
      { title: '店铺', dataIndex: 'shopName', width: 140, render: (v?: string) => v || '—' },
      {
        title: '回款金额',
        dataIndex: 'amount',
        width: 140,
        align: 'right',
        render: (v: number, r) => <span style={tabularNumsStyle}>{`${formatAmount(v)} ${r.currency}`}</span>,
      },
      {
        title: '手续费',
        dataIndex: 'feeAmount',
        width: 110,
        align: 'right',
        render: (v: number) => <span style={tabularNumsStyle}>{formatAmount(v)}</span>,
      },
      { title: '回款日期', dataIndex: 'receivedAt', width: 120, render: (v: string) => (v ? dayjs(v).format('YYYY-MM-DD') : '—') },
      {
        title: '对账状态',
        dataIndex: 'settlementStatus',
        width: 110,
        render: (v: SettlementStatus) => {
          const s = SETTLEMENT_LABEL[v] ?? { text: v, color: 'default' };
          return <Tag color={s.color}>{s.text}</Tag>;
        },
      },
      {
        title: '回款差异',
        dataIndex: 'diffAmount',
        width: 120,
        align: 'right',
        render: (v: number) => <span style={tabularNumsStyle}>{formatAmount(v)}</span>,
      },
      { title: '渠道', dataIndex: 'channel', width: 120, render: (v?: string) => v || '—' },
      {
        title: '来源',
        dataIndex: 'source',
        width: 90,
        render: (v: string) => (v === 'import' ? <Tag>导入</Tag> : <Tag color="blue">手工</Tag>),
      },
    ];
    if (writable) {
      cols.push({
        title: '操作',
        key: 'action',
        width: 90,
        render: (_, r) => (
          <Popconfirm title="确认删除该回款记录？" onConfirm={() => remove(r.id)}>
            <Typography.Link type="danger">删除</Typography.Link>
          </Popconfirm>
        ),
      });
    }
    return cols;
  }, [writable, remove]);

  return (
    <Card size="small">
      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          value={status}
          onChange={(v) => {
            setStatus(v);
            setPage(1);
          }}
          options={STATUS_OPTIONS}
          style={{ width: 140 }}
        />
        <Select
          allowClear
          placeholder="全部店铺"
          value={shopId}
          onChange={(v) => {
            setShopId(v);
            setPage(1);
          }}
          options={shops.map((s) => ({ label: s.shopName, value: s.id }))}
          style={{ width: 180 }}
        />
        {writable ? (
          <>
            <Button type="primary" onClick={() => setOpen(true)}>
              登记回款
            </Button>
            <Button icon={<UploadOutlined />} onClick={() => history.push('/settings/migration?kind=payment')}>
              CSV 批量导入
            </Button>
          </>
        ) : null}
      </Space>
      {!writable ? (
        <Alert type="info" showIcon style={{ marginBottom: 16 }} message="当前账号为只读权限，仅可查看回款记录" />
      ) : null}
      {error ? (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message="回款记录加载失败"
          description={error}
          action={<Button size="small" onClick={load}>重试</Button>}
        />
      ) : null}
      <Table<PaymentRecordRow>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={rows}
        scroll={{ x: 'max-content' }}
        locale={{ emptyText: <EmptyState description="暂无回款记录，可登记回款或使用 CSV 批量导入" /> }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: (p, ps) => {
            setPage(p);
            setPageSize(ps);
          },
        }}
      />
      <Modal
        title="登记回款"
        open={open}
        onOk={submit}
        confirmLoading={saving}
        onCancel={() => setOpen(false)}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" initialValues={{ receivedAt: dayjs() }}>
          <Form.Item name="orderNo" label="订单号" rules={[{ required: true, message: '请输入订单号' }]}>
            <Input placeholder="平台订单号 / 系统订单号" maxLength={64} />
          </Form.Item>
          <Form.Item name="amount" label="回款金额" rules={[{ required: true, message: '请输入回款金额' }]}>
            <InputNumber min={0.01} precision={2} style={{ width: '100%' }} placeholder="平台实际打款金额" />
          </Form.Item>
          <Form.Item name="currency" label="币种" tooltip="留空默认使用订单币种">
            <Input placeholder="如 USD / CNY" maxLength={8} />
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
    </Card>
  );
}

type ShopExpenseFormValues = {
  shopId: string;
  month: Dayjs;
  typeCode: string;
  amount: number;
  currency?: string;
  remark?: string;
};

function ShopExpensesTab({ writable }: { writable: boolean }) {
  const [rows, setRows] = useState<ShopExpenseRow[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [shopId, setShopId] = useState<string | undefined>(undefined);
  const [month, setMonth] = useState<Dayjs | null>(null);
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [types, setTypes] = useState<ExpenseType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm<ShopExpenseFormValues>();

  useEffect(() => {
    queryShops({ page: 1, pageSize: 100 })
      .then((res) => setShops(res.list ?? []))
      .catch(() => setShops([]));
    queryExpenseTypes()
      .then((res) => setTypes(res.items ?? []))
      .catch(() => setTypes([]));
  }, []);

  const load = useCallback(() => {
    setLoading(true);
    setError(null);
    queryShopExpenses({ page, pageSize, shopId, month: month ? month.format('YYYY-MM') : undefined })
      .then((res) => {
        setRows(res.items ?? []);
        setTotal(res.total ?? 0);
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : '加载失败，请稍后重试'))
      .finally(() => setLoading(false));
  }, [page, pageSize, shopId, month]);

  useEffect(() => {
    load();
  }, [load]);

  const submit = useCallback(async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await createShopExpense({
        shopId: values.shopId,
        month: values.month.format('YYYY-MM'),
        typeCode: values.typeCode,
        amount: values.amount,
        currency: values.currency?.trim() || undefined,
        remark: values.remark?.trim() || undefined,
      });
      message.success('店铺费用登记成功');
      setOpen(false);
      form.resetFields();
      load();
    } catch (e) {
      message.error(e instanceof Error ? e.message : '登记失败，请稍后重试');
    } finally {
      setSaving(false);
    }
  }, [form, load]);

  const remove = useCallback(
    (id: string) => {
      deleteShopExpense(id)
        .then(() => {
          message.success('已删除店铺费用');
          load();
        })
        .catch((e: unknown) => message.error(e instanceof Error ? e.message : '删除失败，请稍后重试'));
    },
    [load],
  );

  const columns = useMemo<ColumnsType<ShopExpenseRow>>(() => {
    const cols: ColumnsType<ShopExpenseRow> = [
      { title: '店铺', dataIndex: 'shopName', width: 160, render: (v?: string) => v || '—' },
      { title: '月份', dataIndex: 'month', width: 100 },
      { title: '费用类型', dataIndex: 'typeLabel', width: 130, render: (v: string, r) => v || r.typeCode },
      {
        title: '金额',
        dataIndex: 'amount',
        width: 140,
        align: 'right',
        render: (v: number, r) => <span style={tabularNumsStyle}>{`${formatAmount(v)} ${r.currency}`}</span>,
      },
      { title: '备注', dataIndex: 'remark', ellipsis: true, render: (v?: string) => v || '—' },
    ];
    if (writable) {
      cols.push({
        title: '操作',
        key: 'action',
        width: 90,
        render: (_, r) => (
          <Popconfirm title="确认删除该店铺费用？" onConfirm={() => remove(r.id)}>
            <Typography.Link type="danger">删除</Typography.Link>
          </Popconfirm>
        ),
      });
    }
    return cols;
  }, [writable, remove]);

  return (
    <Card size="small">
      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          allowClear
          placeholder="全部店铺"
          value={shopId}
          onChange={(v) => {
            setShopId(v);
            setPage(1);
          }}
          options={shops.map((s) => ({ label: s.shopName, value: s.id }))}
          style={{ width: 180 }}
        />
        <DatePicker
          picker="month"
          placeholder="全部月份"
          value={month}
          onChange={(v) => {
            setMonth(v);
            setPage(1);
          }}
        />
        {writable ? (
          <Button type="primary" onClick={() => setOpen(true)}>
            登记店铺月度费用
          </Button>
        ) : null}
      </Space>
      {error ? (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message="店铺费用加载失败"
          description={error}
          action={<Button size="small" onClick={load}>重试</Button>}
        />
      ) : null}
      <Table<ShopExpenseRow>
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={rows}
        scroll={{ x: 'max-content' }}
        locale={{ emptyText: <EmptyState description="暂无店铺月度费用" /> }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: (p, ps) => {
            setPage(p);
            setPageSize(ps);
          },
        }}
      />
      <Modal
        title="登记店铺月度费用"
        open={open}
        onOk={submit}
        confirmLoading={saving}
        onCancel={() => setOpen(false)}
        destroyOnHidden
      >
        <Form form={form} layout="vertical">
          <Form.Item name="shopId" label="店铺" rules={[{ required: true, message: '请选择店铺' }]}>
            <Select options={shops.map((s) => ({ label: s.shopName, value: s.id }))} placeholder="选择店铺" />
          </Form.Item>
          <Form.Item name="month" label="月份" rules={[{ required: true, message: '请选择月份' }]}>
            <DatePicker picker="month" style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="typeCode" label="费用类型" rules={[{ required: true, message: '请选择费用类型' }]}>
            <Select options={types.map((t) => ({ label: t.label, value: t.code }))} placeholder="平台佣金 / 推广费 / 运费 / 其他" />
          </Form.Item>
          <Form.Item name="amount" label="费用金额" rules={[{ required: true, message: '请输入费用金额' }]}>
            <InputNumber min={0.01} precision={2} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="currency" label="币种" tooltip="留空默认 CNY">
            <Input placeholder="CNY" maxLength={8} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} maxLength={512} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}

/** 回款与费用：平台回款登记 / CSV 导入 + 店铺级月度费用记账 */
export default function FinancePayments() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: { role?: string } };
  };
  const writable = canWriteOrders(initialState?.currentUser?.role);

  return (
    <TmPageContainer title="回款与费用" subTitle="登记平台回款与店铺月度费用，回款差异自动标记">
      <Tabs
        defaultActiveKey="payments"
        items={[
          { key: 'payments', label: '回款记录', children: <PaymentsTab writable={writable} /> },
          { key: 'shop-expenses', label: '店铺月度费用', children: <ShopExpensesTab writable={writable} /> },
        ]}
      />
    </TmPageContainer>
  );
}
