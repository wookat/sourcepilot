import { TmPageContainer } from '@/components/ui';
import {
  commitImport,
  deleteImportMappingPreset,
  downloadExportCsv,
  downloadImportErrorsCsv,
  downloadImportTemplateCsv,
  downloadImportTemplateXlsx,
  getImportJob,
  getImportProgress,
  parseImportFile,
  queryImportJobs,
  queryImportMappingPresets,
  saveImportMappingPreset,
  validateImport,
  type ImportFieldDef,
  type ImportJobErrorRow,
  type ImportJobRow,
  type ImportKind,
  type ImportMappingPreset,
  type ImportParseResult,
  type ImportValidateResult,
} from '@/services/imports';
import { queryShops, type ShopListRow } from '@/services/shops';
import { canWriteOrders } from '@/utils/permission';
import { formatDateTime } from '@/utils/formatTime';
import { DownloadOutlined, InboxOutlined } from '@ant-design/icons';
import { useModel, useSearchParams } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Drawer,
  Empty,
  Input,
  Popconfirm,
  Progress,
  Radio,
  Result,
  Select,
  Space,
  Steps,
  Table,
  Tag,
  Tabs,
  Typography,
  Upload,
  message,
} from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

const KIND_LABEL: Record<string, string> = {
  product: '商品',
  order: '订单',
  inventory: '库存期初',
  source: '货源档案',
};
const KIND_NEEDS_SHOP: Record<ImportKind, boolean> = {
  product: true,
  order: true,
  inventory: false,
  source: false,
};
const IMPORT_KINDS: ImportKind[] = ['product', 'order', 'inventory', 'source'];
const GROUP_DESC: Record<ImportKind, (n: number) => string> = {
  product: (n) => `将创建 ${n} 个商品草稿；有问题的行不会入库`,
  order: (n) => `将合并为 ${n} 个订单；有问题的行不会入库`,
  inventory: (n) => `将写入 ${n} 条期初库存（含库存流水）；有问题的行不会入库`,
  source: (n) => `将写入 ${n} 条货源档案与 SKU 映射；有问题的行不会入库`,
};
const SOURCE_LABEL: Record<string, string> = {
  dianxiaomi: '店小秘',
  mabang: '马帮',
  custom: '自定义',
};
const JOB_STATUS: Record<string, { text: string; color: string }> = {
  success: { text: '导入成功', color: 'green' },
  partial_success: { text: '部分成功', color: 'orange' },
  failed: { text: '导入失败', color: 'red' },
};
const ROW_STATUS: Record<string, { text: string; color: string }> = {
  failed: { text: '校验失败', color: 'red' },
  duplicate: { text: '重复跳过', color: 'orange' },
};

type WizardStep = 0 | 1 | 2 | 3;

function ImportWizard({ writable }: { writable: boolean }) {
  const [searchParams] = useSearchParams();
  const paramKind = searchParams.get('kind');
  const initialKind: ImportKind = IMPORT_KINDS.includes(paramKind as ImportKind)
    ? (paramKind as ImportKind)
    : 'product';
  const [kind, setKind] = useState<ImportKind>(initialKind);
  const [step, setStep] = useState<WizardStep>(0);
  const [parsing, setParsing] = useState(false);
  const [parsed, setParsed] = useState<ImportParseResult | null>(null);
  const [mapping, setMapping] = useState<Record<string, number>>({});
  const [shops, setShops] = useState<ShopListRow[]>([]);
  const [shopId, setShopId] = useState<string>('');
  const [validating, setValidating] = useState(false);
  const [validated, setValidated] = useState<ImportValidateResult | null>(null);
  const [committing, setCommitting] = useState(false);
  const [commitProgress, setCommitProgress] = useState<{ processed: number; total: number } | null>(null);
  const progressTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const [committed, setCommitted] = useState<{
    jobId: string;
    status: string;
    totalRows: number;
    successRows: number;
    failedRows: number;
    duplicateRows: number;
    replayed: boolean;
  } | null>(null);
  const [presets, setPresets] = useState<ImportMappingPreset[]>([]);
  const [selectedPresetId, setSelectedPresetId] = useState<string | undefined>(undefined);
  const [presetName, setPresetName] = useState('');
  const [savingPreset, setSavingPreset] = useState(false);
  const needsShop = KIND_NEEDS_SHOP[kind];

  const loadPresets = useCallback((k: ImportKind) => {
    queryImportMappingPresets(k)
      .then((res) => setPresets(res.list || []))
      .catch(() => setPresets([]));
  }, []);

  useEffect(() => {
    setSelectedPresetId(undefined);
    loadPresets(kind);
  }, [kind, loadPresets]);

  useEffect(() => {
    let alive = true;
    queryShops({ page: 1, pageSize: 500 })
      .then((res) => {
        if (alive) setShops(res.list || []);
      })
      .catch(() => {
        if (alive) setShops([]);
      });
    return () => {
      alive = false;
    };
  }, []);

  const reset = useCallback(() => {
    setStep(0);
    setParsed(null);
    setMapping({});
    setValidated(null);
    setCommitted(null);
  }, []);

  const fields: ImportFieldDef[] = parsed?.fields || [];

  const handleUpload = useCallback(
    async (file: File) => {
      setParsing(true);
      try {
        const res = await parseImportFile(kind, file);
        setParsed(res);
        setMapping(res.mapping || {});
        setValidated(null);
        setCommitted(null);
        setStep(1);
      } catch (e) {
        message.error(e instanceof Error ? e.message : '文件解析失败');
      } finally {
        setParsing(false);
      }
      return false;
    },
    [kind],
  );

  const wizardBody = useMemo(() => {
    if (!parsed) return null;
    return {
      kind,
      shopId,
      columns: parsed.columns,
      rows: parsed.rows,
      mapping,
      fileName: parsed.fileName,
      fileHash: parsed.fileHash,
      sourceFormat: parsed.sourceFormat,
    };
  }, [parsed, kind, shopId, mapping]);

  const missingRequired = useMemo(
    () => fields.filter((f) => f.required && (mapping[f.key] ?? -1) < 0).map((f) => f.label),
    [fields, mapping],
  );

  const handleValidate = useCallback(async () => {
    if (!wizardBody) return;
    if (needsShop && !shopId) {
      message.warning('请选择归属店铺');
      return;
    }
    if (missingRequired.length > 0) {
      message.warning(`必填字段未映射：${missingRequired.join('、')}`);
      return;
    }
    setValidating(true);
    try {
      const res = await validateImport(wizardBody);
      setValidated(res);
      setStep(2);
    } catch (e) {
      message.error(e instanceof Error ? e.message : '校验失败');
    } finally {
      setValidating(false);
    }
  }, [wizardBody, needsShop, shopId, missingRequired]);

  const stopProgressPolling = useCallback(() => {
    if (progressTimer.current) {
      clearInterval(progressTimer.current);
      progressTimer.current = null;
    }
    setCommitProgress(null);
  }, []);

  useEffect(() => stopProgressPolling, [stopProgressPolling]);

  const handleCommit = useCallback(async () => {
    if (!wizardBody || committing) return;
    setCommitting(true);
    setCommitProgress({ processed: 0, total: wizardBody.rows.length });
    const fileHash = wizardBody.fileHash || '';
    if (fileHash) {
      progressTimer.current = setInterval(() => {
        getImportProgress(kind, fileHash)
          .then((p) => {
            if (p.active) setCommitProgress({ processed: p.processed, total: p.total });
          })
          .catch(() => {
            // 进度查询失败不影响导入本身
          });
      }, 1000);
    }
    try {
      const res = await commitImport(wizardBody);
      setCommitted(res);
      setStep(3);
      if (res.replayed) {
        message.info('该文件此前已导入过，本次未重复写入');
      }
    } catch (e) {
      message.error(e instanceof Error ? e.message : '导入失败');
    } finally {
      stopProgressPolling();
      setCommitting(false);
    }
  }, [wizardBody, committing, kind, stopProgressPolling]);

  const previewColumns = useMemo(() => {
    if (!parsed) return [];
    return parsed.columns.map((c, i) => ({
      title: c || `列${i + 1}`,
      dataIndex: String(i),
      ellipsis: true,
      width: 140,
      render: (_: unknown, row: string[]) =>
        (row[i] ?? '') === '' ? (
          <Typography.Text type="secondary">—</Typography.Text>
        ) : (
          row[i]
        ),
    }));
  }, [parsed]);

  if (!writable) {
    return (
      <Alert
        type="warning"
        showIcon
        message="当前账号为只读权限"
        description="迁移导入需要操作权限，请联系管理员分配。"
      />
    );
  }

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="large">
      <Steps
        current={step}
        items={[{ title: '上传文件' }, { title: '列映射' }, { title: '校验报告' }, { title: '导入结果' }]}
        responsive
      />

      {step === 0 && (
        <Card>
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Space wrap>
              <Typography.Text>导入类型：</Typography.Text>
              <Radio.Group
                value={kind}
                onChange={(e) => setKind(e.target.value)}
                options={[
                  { label: '商品（导入为草稿）', value: 'product' },
                  { label: '历史订单', value: 'order' },
                  { label: '库存期初', value: 'inventory' },
                  { label: '货源档案', value: 'source' },
                ]}
                optionType="button"
              />
            </Space>
            <Alert
              type="info"
              showIcon
              message="不是店小秘 / 马帮导出文件？下载通用模板，按模板整理后上传即可"
              action={
                <Space>
                  <Button
                    size="small"
                    icon={<DownloadOutlined />}
                    onClick={() =>
                      downloadImportTemplateCsv(kind).catch((e) =>
                        message.error(e instanceof Error ? e.message : '模板下载失败'),
                      )
                    }
                  >
                    下载{KIND_LABEL[kind]}模板（CSV）
                  </Button>
                  <Button
                    size="small"
                    icon={<DownloadOutlined />}
                    onClick={() =>
                      downloadImportTemplateXlsx(kind).catch((e) =>
                        message.error(e instanceof Error ? e.message : '模板下载失败'),
                      )
                    }
                  >
                    下载{KIND_LABEL[kind]}模板（Excel）
                  </Button>
                </Space>
              }
            />
            <Upload.Dragger
              accept=".csv,.xlsx"
              maxCount={1}
              showUploadList={false}
              disabled={parsing}
              beforeUpload={(file) => {
                void handleUpload(file);
                return false;
              }}
            >
              <p className="ant-upload-drag-icon">
                <InboxOutlined />
              </p>
              <p className="ant-upload-text">{parsing ? '解析中…' : '点击或拖拽上传 CSV / XLSX 文件'}</p>
              <p className="ant-upload-hint">
                支持店小秘 / 马帮导出文件与通用模板，自动识别列；单批最多 10000 行，文件不超过 10MB
              </p>
            </Upload.Dragger>
          </Space>
        </Card>
      )}

      {step === 1 && parsed && (
        <Card>
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Space wrap>
              <Tag color="blue">{KIND_LABEL[kind]}导入</Tag>
              <Tag>{SOURCE_LABEL[parsed.sourceFormat] || '自定义'}格式</Tag>
              <Typography.Text type="secondary">
                {parsed.fileName} · 共 {parsed.totalRows} 行
              </Typography.Text>
            </Space>
            {needsShop && (
              <Space wrap>
                <Typography.Text>
                  归属店铺<Typography.Text type="danger">*</Typography.Text>：
                </Typography.Text>
                <Select
                  style={{ minWidth: 240 }}
                  placeholder="请选择归属店铺"
                  value={shopId || undefined}
                  onChange={(v) => setShopId(v)}
                  showSearch
                  optionFilterProp="label"
                  options={shops.map((s) => ({ value: s.id, label: s.shopName }))}
                  notFoundContent={<Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无可选店铺" />}
                />
              </Space>
            )}
            <Space wrap>
              <Typography.Text>映射方案：</Typography.Text>
              <Select
                style={{ minWidth: 220 }}
                placeholder={presets.length > 0 ? '选择已保存的映射方案' : '暂无已保存方案'}
                allowClear
                value={selectedPresetId}
                onChange={(id?: string) => {
                  setSelectedPresetId(id);
                  const p = presets.find((x) => x.id === id);
                  if (p) {
                    setMapping(p.mapping || {});
                    message.success(`已套用映射方案「${p.name}」`);
                  }
                }}
                options={presets.map((p) => ({ value: p.id, label: p.name }))}
                notFoundContent={<Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无已保存方案" />}
              />
              <Input
                style={{ width: 180 }}
                placeholder="方案名称（如：店小秘库存）"
                value={presetName}
                maxLength={64}
                onChange={(e) => setPresetName(e.target.value)}
              />
              <Button
                loading={savingPreset}
                onClick={async () => {
                  if (!presetName.trim()) {
                    message.warning('请先填写方案名称');
                    return;
                  }
                  setSavingPreset(true);
                  try {
                    await saveImportMappingPreset({
                      kind,
                      name: presetName.trim(),
                      columns: parsed.columns,
                      mapping,
                    });
                    message.success('映射方案已保存，下次导入可直接套用');
                    setPresetName('');
                    loadPresets(kind);
                  } catch (e) {
                    message.error(e instanceof Error ? e.message : '保存失败');
                  } finally {
                    setSavingPreset(false);
                  }
                }}
              >
                保存当前映射
              </Button>
              {selectedPresetId && (
                <Popconfirm
                  title="删除映射方案"
                  description={`删除方案「${presets.find((p) => p.id === selectedPresetId)?.name ?? ''}」？`}
                  onConfirm={() =>
                    deleteImportMappingPreset(selectedPresetId)
                      .then(() => {
                        message.success('已删除');
                        setSelectedPresetId(undefined);
                        loadPresets(kind);
                      })
                      .catch((e) => message.error(e instanceof Error ? e.message : '删除失败'))
                  }
                >
                  <Button danger type="text" size="small">
                    删除方案
                  </Button>
                </Popconfirm>
              )}
            </Space>
            <Table
              size="small"
              rowKey={(f: ImportFieldDef) => f.key}
              dataSource={fields}
              pagination={false}
              scroll={{ x: 'max-content' }}
              columns={[
                {
                  title: '目标字段',
                  dataIndex: 'label',
                  render: (label: string, f: ImportFieldDef) => (
                    <Space size={4}>
                      {label}
                      {f.required ? <Typography.Text type="danger">*</Typography.Text> : null}
                    </Space>
                  ),
                },
                {
                  title: '来源列',
                  dataIndex: 'key',
                  render: (key: string) => (
                    <Select
                      style={{ minWidth: 200 }}
                      value={(mapping[key] ?? -1) >= 0 ? mapping[key] : undefined}
                      placeholder="未映射"
                      allowClear
                      onChange={(v) => setMapping((m) => ({ ...m, [key]: v ?? -1 }))}
                      options={parsed.columns.map((c, i) => ({ value: i, label: c || `列${i + 1}` }))}
                    />
                  ),
                },
              ]}
            />
            {missingRequired.length > 0 && (
              <Alert type="warning" showIcon message={`必填字段未映射：${missingRequired.join('、')}`} />
            )}
            <Typography.Title level={5} style={{ marginBottom: 0 }}>
              数据预览（前 5 行）
            </Typography.Title>
            <Table
              size="small"
              rowKey="__rowKey"
              dataSource={parsed.rows.slice(0, 5).map((r, i) => Object.assign([...r], { __rowKey: `row-${i}` }))}
              columns={previewColumns}
              pagination={false}
              scroll={{ x: 'max-content' }}
            />
            <Space>
              <Button onClick={reset}>重新上传</Button>
              <Button type="primary" loading={validating} onClick={handleValidate}>
                下一步：校验
              </Button>
            </Space>
          </Space>
        </Card>
      )}

      {step === 2 && validated && (
        <Card>
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Alert
              type={validated.errorRows === 0 ? 'success' : 'warning'}
              showIcon
              message={`共 ${validated.totalRows} 行：可导入 ${validated.validRows} 行，存在问题 ${validated.errorRows} 行`}
              description={GROUP_DESC[kind](validated.groupCount)}
            />
            {validated.errors.length > 0 && (
              <Table
                size="small"
                rowKey="__rowKey"
                dataSource={validated.errors.map((e, i) => ({ ...e, __rowKey: `${e.rowNumber}-${i}` }))}
                pagination={{ pageSize: 10, showSizeChanger: false }}
                scroll={{ x: 'max-content' }}
                columns={[
                  { title: '行号', dataIndex: 'rowNumber', width: 80 },
                  { title: '问题描述', dataIndex: 'message' },
                ]}
              />
            )}
            {committing && commitProgress && (
              <div data-testid="import-commit-progress">
                <Progress
                  percent={
                    commitProgress.total > 0
                      ? Math.round((commitProgress.processed / commitProgress.total) * 100)
                      : 0
                  }
                  status="active"
                />
                <Typography.Text type="secondary">
                  正在导入：{commitProgress.processed} / {commitProgress.total} 行，请勿关闭或重复提交
                </Typography.Text>
              </div>
            )}
            <Space>
              <Button disabled={committing} onClick={() => setStep(1)}>
                返回调整映射
              </Button>
              <Button
                type="primary"
                loading={committing}
                disabled={validated.validRows === 0 || committing}
                onClick={handleCommit}
              >
                {committing ? '导入中…' : `确认导入 ${validated.validRows} 行`}
              </Button>
            </Space>
          </Space>
        </Card>
      )}

      {step === 3 && committed && (
        <Card>
          <Result
            status={committed.status === 'failed' ? 'error' : committed.failedRows > 0 ? 'warning' : 'success'}
            title={
              committed.replayed
                ? '该批次此前已导入（幂等跳过）'
                : committed.status === 'failed'
                  ? '导入失败'
                  : committed.failedRows > 0
                    ? `部分成功：成功 ${committed.successRows} 行，失败 ${committed.failedRows} 行`
                    : `导入成功：共 ${committed.successRows} 行`
            }
            subTitle={
              committed.failedRows > 0 && committed.status !== 'failed'
                ? `共 ${committed.totalRows} 行 · 成功 ${committed.successRows} · 失败 ${committed.failedRows} · 重复跳过 ${committed.duplicateRows}；失败行未入库，可下载错误行报告修正后重新导入`
                : `共 ${committed.totalRows} 行 · 成功 ${committed.successRows} · 失败 ${committed.failedRows} · 重复跳过 ${committed.duplicateRows}`
            }
            extra={[
              (committed.failedRows > 0 || committed.duplicateRows > 0) && (
                <Button
                  key="errors"
                  onClick={() =>
                    downloadImportErrorsCsv(committed.jobId).catch((e) =>
                      message.error(e instanceof Error ? e.message : '下载失败'),
                    )
                  }
                >
                  下载错误行报告
                </Button>
              ),
              <Button key="again" type="primary" onClick={reset}>
                继续导入
              </Button>,
            ].filter(Boolean)}
          />
        </Card>
      )}
    </Space>
  );
}

function ImportHistory({ onGoWizard, refreshToken }: { onGoWizard: () => void; refreshToken: number }) {
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<ImportJobRow[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [kind, setKind] = useState<string>('');
  const [detail, setDetail] = useState<{ job: ImportJobRow; errorRows: ImportJobErrorRow[] } | null>(null);

  const load = useCallback(async (p: number, ps: number, k: string) => {
    setLoading(true);
    try {
      const res = await queryImportJobs({ page: p, pageSize: ps, kind: k || undefined });
      setRows(res.list || []);
      setTotal(res.total || 0);
    } catch (e) {
      message.error(e instanceof Error ? e.message : '加载失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load(page, pageSize, kind);
  }, [load, page, pageSize, kind, refreshToken]);

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      <Radio.Group
        value={kind}
        onChange={(e) => {
          setKind(e.target.value);
          setPage(1);
        }}
        options={[
          { label: '全部', value: '' },
          { label: '商品', value: 'product' },
          { label: '订单', value: 'order' },
          { label: '库存期初', value: 'inventory' },
          { label: '货源档案', value: 'source' },
        ]}
        optionType="button"
      />
      <Table
        size="small"
        loading={loading}
        rowKey="id"
        dataSource={rows}
        scroll={{ x: 'max-content' }}
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
        locale={{
          emptyText: (
            <Empty description="暂无导入记录，从导入向导上传店小秘 / 马帮导出文件或通用模板开始迁移">
              <Button type="primary" onClick={onGoWizard}>
                去导入向导
              </Button>
            </Empty>
          ),
        }}
        columns={[
          {
            title: '类型',
            dataIndex: 'kind',
            width: 80,
            render: (k: string) => KIND_LABEL[k] || k,
          },
          { title: '文件名', dataIndex: 'fileName', ellipsis: true },
          {
            title: '来源格式',
            dataIndex: 'sourceFormat',
            width: 100,
            render: (s: string) => SOURCE_LABEL[s] || '自定义',
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 100,
            render: (s: string) => {
              const c = JOB_STATUS[s];
              return c ? <Tag color={c.color}>{c.text}</Tag> : <Tag>{s}</Tag>;
            },
          },
          { title: '总行数', dataIndex: 'totalRows', width: 90 },
          { title: '成功', dataIndex: 'successRows', width: 80 },
          { title: '失败', dataIndex: 'failedRows', width: 80 },
          { title: '重复', dataIndex: 'duplicateRows', width: 80 },
          {
            title: '导入时间',
            dataIndex: 'createdAt',
            width: 170,
            render: (v: string) => formatDateTime(v),
          },
          {
            title: '操作',
            key: 'ops',
            width: 200,
            render: (_: unknown, row: ImportJobRow) => (
              <Space>
                <Button
                  size="small"
                  onClick={() =>
                    getImportJob(row.id)
                      .then(setDetail)
                      .catch((e) => message.error(e instanceof Error ? e.message : '加载失败'))
                  }
                >
                  查看
                </Button>
                {row.errorRowCount > 0 && (
                  <Button
                    size="small"
                    onClick={() =>
                      downloadImportErrorsCsv(row.id).catch((e) =>
                        message.error(e instanceof Error ? e.message : '下载失败'),
                      )
                    }
                  >
                    错误行下载
                  </Button>
                )}
              </Space>
            ),
          },
        ]}
      />
      <Drawer
        open={!!detail}
        onClose={() => setDetail(null)}
        title={detail ? `导入详情 · ${detail.job.fileName || KIND_LABEL[detail.job.kind]}` : '导入详情'}
        width={720}
      >
        {detail && (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Space wrap>
              <Tag color="blue">{KIND_LABEL[detail.job.kind]}</Tag>
              <Tag>{SOURCE_LABEL[detail.job.sourceFormat] || '自定义'}</Tag>
              {(() => {
                const c = JOB_STATUS[detail.job.status];
                return c ? <Tag color={c.color}>{c.text}</Tag> : null;
              })()}
              <Typography.Text type="secondary">
                共 {detail.job.totalRows} 行 · 成功 {detail.job.successRows} · 失败 {detail.job.failedRows} · 重复{' '}
                {detail.job.duplicateRows}
              </Typography.Text>
            </Space>
            {detail.errorRows.length === 0 ? (
              <Empty description="没有错误行" />
            ) : (
              <Table
                size="small"
                rowKey="id"
                dataSource={detail.errorRows}
                pagination={{ pageSize: 10, showSizeChanger: false }}
                scroll={{ x: 'max-content' }}
                columns={[
                  { title: '行号', dataIndex: 'rowNumber', width: 80 },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    width: 100,
                    render: (s: string) => {
                      const c = ROW_STATUS[s];
                      return c ? <Tag color={c.color}>{c.text}</Tag> : <Tag>{s}</Tag>;
                    },
                  },
                  { title: '问题描述', dataIndex: 'message' },
                ]}
              />
            )}
          </Space>
        )}
      </Drawer>
    </Space>
  );
}

function ExportCenter() {
  const [downloading, setDownloading] = useState<ImportKind | null>(null);
  const EXPORT_DESC: Record<ImportKind, string> = {
    product: '全部商品草稿，每个 SKU 一行，含售价 / 成本 / 库存与商品币种',
    order: '全部订单，每个商品行一行，含订单币种与金额口径',
    inventory: '全部 SKU 库存，按仓库逐行展开，含默认仓与各仓库存',
    source: '全部货源档案与 SKU 映射，含供应商 / 链接 / 参考价',
  };
  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      <Alert
        type="info"
        showIcon
        message="数据搬出"
        description="四类数据均可全量导出为 CSV（UTF-8，Excel 可直接打开），列口径与导入模板兼容，便于迁入其他系统。"
      />
      <Table
        size="small"
        rowKey="kind"
        pagination={false}
        scroll={{ x: 'max-content' }}
        dataSource={IMPORT_KINDS.map((k) => ({ kind: k }))}
        columns={[
          {
            title: '数据类型',
            dataIndex: 'kind',
            width: 140,
            render: (k: ImportKind) => <Tag color="blue">{KIND_LABEL[k]}</Tag>,
          },
          {
            title: '导出内容',
            dataIndex: 'kind',
            key: 'desc',
            render: (k: ImportKind) => EXPORT_DESC[k],
          },
          {
            title: '操作',
            dataIndex: 'kind',
            key: 'ops',
            width: 160,
            render: (k: ImportKind) => (
              <Button
                size="small"
                icon={<DownloadOutlined />}
                loading={downloading === k}
                onClick={async () => {
                  setDownloading(k);
                  try {
                    await downloadExportCsv(k);
                  } catch (e) {
                    message.error(e instanceof Error ? e.message : '导出失败');
                  } finally {
                    setDownloading(null);
                  }
                }}
              >
                导出 CSV
              </Button>
            ),
          },
        ]}
      />
    </Space>
  );
}

export default function MigrationImportPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const writable = canWriteOrders(
    initialState?.currentUser?.role,
    initialState?.currentUser?.permissions,
  );
  const [activeTab, setActiveTab] = useState('wizard');
  // Tabs keep children mounted after first visit; bump the token whenever the
  // history tab is (re)activated so newly committed jobs show without a manual
  // page refresh.
  const [historyRefreshToken, setHistoryRefreshToken] = useState(0);
  const switchTab = (key: string) => {
    if (key === 'history') setHistoryRefreshToken((n) => n + 1);
    setActiveTab(key);
  };

  return (
    <TmPageContainer
      title="数据搬家"
      subTitle="支持店小秘 / 马帮与通用模板导入商品、订单、库存期初、货源档案，并可全量导出"
    >
      <Tabs
        activeKey={activeTab}
        onChange={switchTab}
        items={[
          { key: 'wizard', label: '导入向导', children: <ImportWizard writable={writable} /> },
          {
            key: 'history',
            label: '导入历史',
            children: (
              <ImportHistory onGoWizard={() => switchTab('wizard')} refreshToken={historyRefreshToken} />
            ),
          },
          { key: 'export', label: '数据导出', children: <ExportCenter /> },
        ]}
      />
    </TmPageContainer>
  );
}
