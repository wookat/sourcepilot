import { TmPageContainer } from '@/components/ui';
import {
  bannedWordLevelLabel,
  createBannedWord,
  deleteBannedWord,
  listBannedWordCategories,
  listBannedWords,
  toggleBannedWordCategory,
  updateBannedWord,
  type BannedWordCategory,
  type BannedWordRow,
} from '@/services/bannedWords';
import { isReadonly } from '@/utils/permission';
import { useModel } from '@umijs/max';
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd';
import { useCallback, useEffect, useState } from 'react';

const LEVEL_OPTIONS = [
  { value: 'forbidden', label: '禁止（阻断刊登）' },
  { value: 'warning', label: '警告（仅提示）' },
];

export default function BannedWordsSettingsPage() {
  const { initialState } = useModel('@@initialState') as {
    initialState?: { currentUser?: API.CurrentUser };
  };
  const readonly = isReadonly(initialState?.currentUser?.role);

  const [rows, setRows] = useState<BannedWordRow[]>([]);
  const [categories, setCategories] = useState<BannedWordCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState('');
  const [keyword, setKeyword] = useState('');
  const [categoryFilter, setCategoryFilter] = useState<string>('');
  const [togglingId, setTogglingId] = useState('');
  const [togglingCategory, setTogglingCategory] = useState('');
  const [modalOpen, setModalOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async (kw?: string, cat?: string) => {
    setLoading(true);
    setLoadError('');
    try {
      const [words, cats] = await Promise.all([
        listBannedWords({ keyword: kw, category: cat || undefined }),
        listBannedWordCategories(),
      ]);
      setRows(words);
      setCategories(cats);
    } catch (e) {
      setLoadError((e as Error).message || '加载违禁词库失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const submit = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      await createBannedWord({
        word: v.word,
        category: v.category,
        level: v.level,
        suggestion: v.suggestion,
      });
      message.success('已新增违禁词');
      setModalOpen(false);
      form.resetFields();
      await load(keyword, categoryFilter);
    } catch (e) {
      message.error((e as Error).message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const toggleWord = async (row: BannedWordRow, enabled: boolean) => {
    setTogglingId(row.id);
    try {
      await updateBannedWord(row.id, { enabled });
      message.success(enabled ? `已启用「${row.word}」` : `已停用「${row.word}」`);
      await load(keyword, categoryFilter);
    } catch (e) {
      message.error((e as Error).message || '操作失败');
    } finally {
      setTogglingId('');
    }
  };

  const toggleCategory = async (cat: BannedWordCategory, enabled: boolean) => {
    setTogglingCategory(cat.category);
    try {
      await toggleBannedWordCategory(cat.category, enabled);
      message.success(enabled ? `已启用分类「${cat.categoryLabel}」` : `已停用分类「${cat.categoryLabel}」`);
      await load(keyword, categoryFilter);
    } catch (e) {
      message.error((e as Error).message || '操作失败');
    } finally {
      setTogglingCategory('');
    }
  };

  const remove = async (row: BannedWordRow) => {
    try {
      await deleteBannedWord(row.id);
      message.success(`已删除「${row.word}」`);
      await load(keyword, categoryFilter);
    } catch (e) {
      message.error((e as Error).message || '删除失败');
    }
  };

  const categoryOptions = categories.map((c) => ({ value: c.category, label: c.categoryLabel }));

  return (
    <TmPageContainer
      title="违禁词库"
      subTitle="刊登前合规检测的词库：预置基础库（广告法极限词、通用违禁词等）只读可启停，租户可增删自定义词；禁止级命中会阻断刊登"
    >
      <Card title="分类启停" style={{ marginBottom: 16 }} loading={loading && categories.length === 0}>
        <Space wrap size={[16, 8]}>
          {categories.map((cat) => (
            <Space key={cat.category} size={8}>
              <Switch
                size="small"
                checked={cat.enabled}
                disabled={readonly}
                loading={togglingCategory === cat.category}
                aria-label={`启停分类 ${cat.categoryLabel}`}
                onChange={(checked) => void toggleCategory(cat, checked)}
              />
              <span>
                {cat.categoryLabel}
                <Tag style={{ marginLeft: 4 }}>{cat.wordCount} 词</Tag>
              </span>
            </Space>
          ))}
          {!loading && categories.length === 0 ? <span>暂无分类</span> : null}
        </Space>
      </Card>
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Input.Search
            allowClear
            placeholder="按违禁词搜索"
            style={{ width: 240 }}
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            onSearch={(v) => void load(v, categoryFilter)}
          />
          <Select
            allowClear
            placeholder="按分类筛选"
            style={{ width: 180 }}
            options={categoryOptions}
            value={categoryFilter || undefined}
            onChange={(v) => {
              setCategoryFilter(v || '');
              void load(keyword, v || '');
            }}
          />
          <Tooltip title={readonly ? '只读账号不可新增违禁词' : ''}>
            <Button type="primary" disabled={readonly} onClick={() => setModalOpen(true)}>
              新增自定义违禁词
            </Button>
          </Tooltip>
        </Space>
        {loadError ? (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            message="加载违禁词库失败"
            description={loadError}
            action={
              <Button size="small" onClick={() => void load(keyword, categoryFilter)}>
                重试
              </Button>
            }
          />
        ) : null}
        <Table<BannedWordRow>
          rowKey="id"
          size="middle"
          loading={loading}
          dataSource={rows}
          pagination={{ pageSize: 20, showSizeChanger: false }}
          scroll={{ x: 760 }}
          locale={{ emptyText: '暂无违禁词，点击「新增自定义违禁词」添加' }}
          columns={[
            { title: '违禁词', dataIndex: 'word', width: 160 },
            {
              title: '分类',
              dataIndex: 'category',
              width: 140,
              render: (v: string) =>
                categories.find((c) => c.category === v)?.categoryLabel || v || '自定义',
            },
            {
              title: '级别',
              dataIndex: 'level',
              width: 100,
              render: (v: string) =>
                v === 'forbidden' ? (
                  <Tag color="red">禁止</Tag>
                ) : (
                  <Tag color="orange">{bannedWordLevelLabel(v)}</Tag>
                ),
            },
            {
              title: '类型',
              dataIndex: 'isPreset',
              width: 90,
              render: (v: boolean) => (v ? <Tag color="blue">预置</Tag> : <Tag>自定义</Tag>),
            },
            {
              title: '建议',
              dataIndex: 'suggestion',
              ellipsis: true,
              render: (v: string) =>
                v ? (
                  <Tooltip title={v} placement="topLeft">
                    <span>{v}</span>
                  </Tooltip>
                ) : (
                  '—'
                ),
            },
            {
              title: '启用',
              dataIndex: 'enabled',
              width: 90,
              render: (v: boolean, row) => (
                <Switch
                  checked={v}
                  size="small"
                  disabled={readonly}
                  loading={togglingId === row.id}
                  aria-label={`启停违禁词 ${row.word}`}
                  onChange={(checked) => void toggleWord(row, checked)}
                />
              ),
            },
            {
              title: '操作',
              width: 100,
              render: (_, row) =>
                row.isPreset ? (
                  <Tooltip title="预置违禁词不可删除，可停用">
                    <Button size="small" type="link" disabled>
                      删除
                    </Button>
                  </Tooltip>
                ) : (
                  <Popconfirm
                    title={`删除违禁词「${row.word}」？`}
                    okText="删除"
                    okButtonProps={{ danger: true }}
                    disabled={readonly}
                    onConfirm={() => void remove(row)}
                  >
                    <Button size="small" type="link" danger disabled={readonly}>
                      删除
                    </Button>
                  </Popconfirm>
                ),
            },
          ]}
        />
      </Card>
      <Modal
        title="新增自定义违禁词"
        open={modalOpen}
        confirmLoading={saving}
        onCancel={() => setModalOpen(false)}
        onOk={() => void submit()}
        forceRender
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="word"
            label="违禁词"
            rules={[
              { required: true, message: '请填写违禁词' },
              { max: 64, message: '不能超过 64 个字符' },
            ]}
          >
            <Input placeholder="如：全网首发" />
          </Form.Item>
          <Form.Item name="category" label="分类" initialValue="custom">
            <Select
              options={[...categoryOptions, { value: 'custom', label: '自定义' }]}
              placeholder="选择分类"
            />
          </Form.Item>
          <Form.Item name="level" label="级别" initialValue="forbidden">
            <Select options={LEVEL_OPTIONS} />
          </Form.Item>
          <Form.Item name="suggestion" label="修改建议（可选）">
            <Input placeholder="如：请改为客观描述" />
          </Form.Item>
        </Form>
      </Modal>
    </TmPageContainer>
  );
}
