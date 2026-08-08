import { extractErrorMessage } from '@/utils/httpErrorCopy';
import { formatDateTime } from '@/utils/formatTime';
import { TmPageContainer } from '@/components/ui';
import { history, useParams, useSearchParams } from '@umijs/max';
import {
  Button,
  Card,
  Col,
  Descriptions,
  Empty,
  Input,
  List,
  message,
  Modal,
  Result,
  Row,
  Select,
  Space,
  Spin,
  Tabs,
  Tag,
  Typography,
  Alert,
} from 'antd';
import dayjs from 'dayjs';
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  CUSTOMER_CONVERSATION_STATUS,
  CUSTOMER_MESSAGE_ROLE_LABEL,
  CUSTOMER_MESSAGE_SOURCE_LABEL,
  CUSTOMER_MESSAGE_TYPE_LABEL,
  ORDER_FULFILLMENT_STATUS,
  ORDER_INVENTORY_DEDUCT_SUMMARY,
  ORDER_PAYMENT_STATUS,
  ORDER_SHIPMENT_STATUS,
  ORDER_SKU_MATCH_SUMMARY,
  ORDER_STATUS,
  SHOP_AUTH_STATUS,
  SHOP_STATUS,
} from '@/constants/status';
import { platformLabel } from '@/constants/userFriendly';
import { confirmCustomerReplySend } from '@/constants/sensitiveActions';
import {
  acceptReplySuggestion,
  createMessage,
  discardReplySuggestion,
  generateCustomerReply,
  getConversation,
  queryMessages,
  queryReplyTemplates,
  querySuggestions,
  sendPlatformMessage,
  updateConversation,
  updateReplySuggestion,
  type ConversationDetail,
  type CustomerMessageRow,
  type GenerateReplyResult,
  type ReplyTemplateGroupKey,
  type ReplyTemplateRow,
  type SuggestionRow,
} from '@/services/customer';
import {
  REPLY_TEMPLATE_GROUPS,
  fillReplyTemplate,
  replyTemplateGroupLabel,
  type ReplyTemplateVarContext,
} from '@/utils/replyTemplateVars';
import { queryOrders, type OrderListRow } from '@/services/orders';
import { queryShops } from '@/services/shops';

function mapBizStatus(raw: string, dictionary: Record<string, { text: string; color: string }>) {
  const k = dictionary[raw as keyof typeof dictionary];
  if (!k) return <Tag>{raw}</Tag>;
  return <Tag color={k.color as never}>{k.text}</Tag>;
}

/** 仍可编辑/采纳的草稿建议（含发送失败待重试） */
function isDraftSuggestion(s: SuggestionRow) {
  const st = (s.status || '').toLowerCase();
  return st === 'generated' || st === 'edited' || st === 'send_failed';
}

function riskTag(level: string) {
  const l = (level || '').toLowerCase();
  if (l === 'high') return <Tag color="error">high</Tag>;
  if (l === 'medium') return <Tag color="warning">medium</Tag>;
  return <Tag color="success">low</Tag>;
}

export default function CustomerConversationDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [searchParams] = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [conv, setConv] = useState<ConversationDetail | null>(null);
  const [loadError, setLoadError] = useState<{ status?: number; message: string } | null>(null);
  const [msgs, setMsgs] = useState<CustomerMessageRow[]>([]);

  const [newCustomerMsg, setNewCustomerMsg] = useState('');
  const [lang, setLang] = useState('en');
  const [tone, setTone] = useState('professional');
  const [platform, setPlatform] = useState('manual');
  const [shopPolicy, setShopPolicy] = useState('');
  const [focusMessageId, setFocusMessageId] = useState<string | undefined>(undefined);

  const [genLoading, setGenLoading] = useState(false);
  const [suggestionId, setSuggestionId] = useState<string | null>(null);
  const [aiMeta, setAiMeta] = useState<Omit<GenerateReplyResult, 'suggestionId' | 'taskId'> | null>(null);
  const [editedReply, setEditedReply] = useState('');

  const [orderPickOpen, setOrderPickOpen] = useState(false);
  const [orderSearchLoading, setOrderSearchLoading] = useState(false);
  const [orderFilterNo, setOrderFilterNo] = useState('');
  const [orderFilterName, setOrderFilterName] = useState('');
  const [orderHits, setOrderHits] = useState<OrderListRow[]>([]);

  const [tplOpen, setTplOpen] = useState(false);
  const [tplLoading, setTplLoading] = useState(false);
  const [tplRows, setTplRows] = useState<ReplyTemplateRow[]>([]);
  const [tplGroup, setTplGroup] = useState<'all' | ReplyTemplateGroupKey>('all');
  const [tplKeyword, setTplKeyword] = useState('');

  const [shopPickOpen, setShopPickOpen] = useState(false);
  const [shopOpts, setShopOpts] = useState<{ label: string; value: string }[]>([]);
  const [pickedShopId, setPickedShopId] = useState<string | undefined>();

  const loadAll = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    try {
      const [c, m, sugg] = await Promise.all([getConversation(id), queryMessages(id), querySuggestions(id)]);
      setLoadError(null);
      setConv(c);
      setMsgs(m.list || []);
      setLang(c.customerLanguage || 'en');
      setPlatform(c.platform || 'manual');
      const drafts = (sugg.list || []).filter(isDraftSuggestion);
      const urlSid = searchParams.get('suggestionId')?.trim();
      const chosen = drafts.find((s) => s.id === urlSid) ?? drafts[drafts.length - 1];
      if (chosen) {
        setSuggestionId((prev) => prev ?? chosen.id);
        setEditedReply((prev) => (prev.trim() ? prev : chosen.editedReply || chosen.suggestedReply || ''));
      }
    } catch (e: unknown) {
      const err = e as { response?: { status?: number }; code?: number };
      const status =
        err?.response?.status ??
        (typeof err?.code === 'number' ? (err.code >= 10000 ? Math.floor(err.code / 100) : err.code) : undefined);
      setLoadError({ status, message: extractErrorMessage(e, '会话加载失败') });
    } finally {
      setLoading(false);
    }
  }, [id, searchParams]);

  const runOrderSearch = useCallback(async () => {
    setOrderSearchLoading(true);
    try {
      const res = await queryOrders({
        page: 1,
        pageSize: 20,
        orderNo: orderFilterNo.trim() || undefined,
        customerName: orderFilterName.trim() || undefined,
      });
      setOrderHits(res.list || []);
    } finally {
      setOrderSearchLoading(false);
    }
  }, [orderFilterName, orderFilterNo]);

  const linkOrder = async (orderId: string) => {
    if (!id) return;
    await updateConversation(id, { orderId });
    message.success('已关联订单');
    setOrderPickOpen(false);
    loadAll();
  };

  const unlinkOrder = async () => {
    if (!id) return;
    Modal.confirm({
      title: '取消关联订单？',
      // onOk 不返回 Promise，由回调手动 close：失败保持弹窗打开且不向外抛拒绝。
      onOk: (close: () => void) => {
        updateConversation(id, { orderId: '' })
          .then(() => {
            message.success('已取消关联');
            loadAll();
            close();
          })
          .catch((e: unknown) => {
            message.error(extractErrorMessage(e, '取消关联失败'));
          });
      },
    });
  };

  const openShopPick = async () => {
    const res = await queryShops({ page: 1, pageSize: 300 });
    setShopOpts(
      res.list.map((s) => ({
        label: `${s.shopName} (${platformLabel(s.platform)})`,
        value: s.id,
      })),
    );
    setPickedShopId(conv?.shopId);
    setShopPickOpen(true);
  };

  const linkShop = async () => {
    if (!id || !pickedShopId) {
      message.warning('请选择店铺');
      return;
    }
    await updateConversation(id, { shopId: pickedShopId });
    message.success('已关联店铺');
    setShopPickOpen(false);
    loadAll();
  };

  const unlinkShop = async () => {
    if (!id) return;
    Modal.confirm({
      title: '取消关联店铺？',
      onOk: (close: () => void) => {
        updateConversation(id, { shopId: '' })
          .then(() => {
            message.success('已取消关联');
            loadAll();
            close();
          })
          .catch((e: unknown) => {
            message.error(extractErrorMessage(e, '取消关联失败'));
          });
      },
    });
  };

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  useEffect(() => {
    const sid = searchParams.get('suggestionId')?.trim();
    if (sid) setSuggestionId(sid);
  }, [searchParams]);

  const customerMessageOptions = useMemo(() => {
    return msgs.filter((x) => x.role === 'customer').map((m) => ({ label: m.content.slice(0, 48) + (m.content.length > 48 ? '…' : ''), value: m.id }));
  }, [msgs]);

  const onAddCustomerMessage = async () => {
    if (!id) return;
    const t = newCustomerMsg.trim();
    if (!t) {
      message.warning('请输入客户消息');
      return;
    }
    try {
      await createMessage(id, { role: 'customer', content: t, language: lang });
    } catch (e: unknown) {
      message.error(extractErrorMessage(e, '添加失败'));
      return;
    }
    setNewCustomerMsg('');
    message.success('已添加');
    loadAll();
  };

  const onGenerate = async () => {
    if (!id) return;
    setGenLoading(true);
    try {
      const res = await generateCustomerReply(id, {
        messageId: focusMessageId,
        language: lang,
        tone,
        platform,
        shopPolicy,
      });
      setSuggestionId(res.suggestionId);
      setAiMeta({
        reply: res.reply,
        intent: res.intent,
        sentiment: res.sentiment,
        riskLevel: res.riskLevel,
        notes: res.notes,
      });
      setEditedReply(res.reply);
      if (searchParams.get('suggestionId')) {
        setSuggestionId(searchParams.get('suggestionId'));
      }
      message.success('已生成建议（需人工确认，不会对外发送）');
      loadAll();
    } catch (e: unknown) {
      message.error(extractErrorMessage(e, '生成失败'));
    } finally {
      setGenLoading(false);
    }
  };

  const templateVarContext: ReplyTemplateVarContext = useMemo(
    () => ({
      买家昵称: conv?.customerNameMasked || conv?.customerName,
      订单号: conv?.orderSummary?.orderNo,
      物流单号: conv?.orderSummary?.shipments?.find((s) => s.trackingNo)?.trackingNo,
      商品名: conv?.contextSummary?.productTitle || conv?.productContexts?.[0]?.productTitle,
      店铺名: conv?.shopSummary?.shopName,
    }),
    [conv],
  );

  const openTemplatePicker = async () => {
    setTplOpen(true);
    setTplLoading(true);
    try {
      const res = await queryReplyTemplates({ enabled: true });
      setTplRows(res.list || []);
    } catch (e: unknown) {
      message.error(extractErrorMessage(e, '加载话术模板失败'));
    } finally {
      setTplLoading(false);
    }
  };

  const visibleTemplates = useMemo(() => {
    const kw = tplKeyword.trim();
    return tplRows.filter(
      (t) =>
        (tplGroup === 'all' || t.groupKey === tplGroup) &&
        (!kw || t.name.includes(kw) || t.content.includes(kw)),
    );
  }, [tplRows, tplGroup, tplKeyword]);

  const insertTemplate = (tpl: ReplyTemplateRow) => {
    const { text, missing } = fillReplyTemplate(tpl.content, templateVarContext);
    setEditedReply((prev) => (prev.trim() ? `${prev}\n${text}` : text));
    setTplOpen(false);
    if (missing.length > 0) {
      message.warning(`已插入模板，但以下变量缺少上下文未填充，请手动补充：${missing.map((m) => `{${m}}`).join('、')}`);
    } else {
      message.success('已插入话术模板，可继续编辑后人工发送');
    }
  };

  const onCopy = async () => {
    const t = editedReply.trim();
    if (!t) {
      message.warning('没有可复制内容');
      return;
    }
    try {
      await navigator.clipboard.writeText(t);
      message.success('已复制');
    } catch {
      message.error('复制失败');
    }
  };

  const onSaveEdit = async () => {
    if (!suggestionId) {
      message.warning('请先生成建议');
      return;
    }
    await updateReplySuggestion(suggestionId, { editedReply: editedReply.trim() });
    message.success('已保存编辑');
  };

  const onAccept = async () => {
    if (!suggestionId) {
      message.warning('请先生成建议');
      return;
    }
    const finalReply = editedReply.trim();
    if (!finalReply) {
      message.warning('回复内容不能为空');
      return;
    }
    await acceptReplySuggestion(suggestionId, { finalReply });
    message.success('已采纳为内部回复（仅记录，不向平台发送）');
    setSuggestionId(null);
    setAiMeta(null);
    setEditedReply('');
    loadAll();
  };

  const onSendToPlatform = () => {
    if (!id) return;
    const finalReply = editedReply.trim();
    if (!finalReply) {
      message.warning('请填写要发送到平台的回复内容');
      return;
    }
    confirmCustomerReplySend(
      canSendToPlatform,
      async () => {
        await sendPlatformMessage(id, {
          reply: finalReply,
          suggestionId: suggestionId || undefined,
        });
        message.success('已发送到平台');
        loadAll();
      },
      (e) => extractErrorMessage(e, '发送失败'),
    );
  };

  const onDiscard = async () => {
    if (!suggestionId) {
      message.warning('没有可废弃的建议');
      return;
    }
    await discardReplySuggestion(suggestionId);
    message.success('已废弃');
    setSuggestionId(null);
    setAiMeta(null);
    setEditedReply('');
  };

  if (!id) {
    return null;
  }

  if (loadError) {
    const forbidden = loadError.status === 403;
    const notFound = loadError.status === 404;
    return (
      <TmPageContainer title="AI 客服工作台" onBack={() => window.history.back()}>
        <Result
          status={forbidden ? '403' : notFound ? '404' : 'error'}
          title={forbidden ? '无权访问该会话' : notFound ? '会话不存在或已被删除' : '会话加载失败'}
          subTitle={forbidden ? '该会话不属于当前账号可访问的租户，请确认账号或联系管理员。' : notFound ? '会话可能已被删除，或链接有误。' : loadError.message}
          extra={
            <Space>
              <Button type="primary" onClick={() => history.push('/customer/conversations')}>
                返回会话列表
              </Button>
              {!forbidden && !notFound ? <Button onClick={() => loadAll()}>重试</Button> : null}
            </Space>
          }
        />
      </TmPageContainer>
    );
  }

  const statusMap = conv
    ? CUSTOMER_CONVERSATION_STATUS[conv.status as keyof typeof CUSTOMER_CONVERSATION_STATUS]
    : undefined;

  const canSendToPlatform = Boolean(conv?.shopId && conv?.externalConversationId && conv?.canWrite !== false);
  const readOnly = conv?.canWrite === false;

  return (
    <TmPageContainer title="AI 客服工作台" onBack={() => window.history.back()}>
      <Spin spinning={loading}>
        {conv && (
          <>
            {conv.openFailureCount ? (
              <Alert
                type="error"
                showIcon
                style={{ marginBottom: 16 }}
                message={`有 ${conv.openFailureCount} 条未处理客服失败`}
                action={
                  <Button size="small" onClick={() => history.push('/ops/task-center/failures?taskType=customer_failure')}>
                    查看失败任务
                  </Button>
                }
              />
            ) : null}
            {readOnly ? (
              <Alert type="info" showIcon style={{ marginBottom: 16 }} message="当前为只读账号，不可生成建议或发送消息。" />
            ) : null}
            {conv.contextSummary?.incompleteWarning ? (
              <Alert type="warning" showIcon style={{ marginBottom: 16 }} message={conv.contextSummary.incompleteWarning} />
            ) : null}
            <Descriptions size="small" column={{ xs: 1, sm: 2, md: 3 }} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="客户">{conv.customerNameMasked || conv.customerName}</Descriptions.Item>
              <Descriptions.Item label="平台">{platformLabel(conv.platform)}</Descriptions.Item>
              <Descriptions.Item label="状态">
                {statusMap ? <Tag color={statusMap.color}>{statusMap.text}</Tag> : <Tag>{conv.status}</Tag>}
              </Descriptions.Item>
              <Descriptions.Item label="店铺">{conv.shopSummary?.shopName ?? '—'}</Descriptions.Item>
              <Descriptions.Item label="外部会话 ID" span={2}>
                <Typography.Text copyable={conv.externalConversationId ? { text: conv.externalConversationId } : false}>
                  {conv.externalConversationId ?? '—'}
                </Typography.Text>
              </Descriptions.Item>
            </Descriptions>
            <Card size="small" title="关联店铺" variant="borderless" style={{ marginBottom: 16 }}>
              <Space direction="vertical" style={{ width: '100%' }}>
                <Space wrap>
                  <Button type="primary" disabled={readOnly} onClick={() => void openShopPick()}>
                    选择店铺
                  </Button>
                  {conv.shopId ? (
                    <Button danger disabled={readOnly} onClick={() => void unlinkShop()}>
                      取消关联
                    </Button>
                  ) : null}
                </Space>
                {conv.shopSummary ? (
                  <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
                    <Descriptions.Item label="店铺名">{conv.shopSummary.shopName}</Descriptions.Item>
                    <Descriptions.Item label="平台">{platformLabel(conv.shopSummary.platform)}</Descriptions.Item>
                    <Descriptions.Item label="店铺状态">
                      {(() => {
                        const m = SHOP_STATUS[conv.shopSummary.status as keyof typeof SHOP_STATUS];
                        return m ? <Tag color={m.color}>{m.text}</Tag> : conv.shopSummary.status || '—';
                      })()}
                    </Descriptions.Item>
                    <Descriptions.Item label="授权状态">
                      {(() => {
                        const m = SHOP_AUTH_STATUS[conv.shopSummary.authStatus as keyof typeof SHOP_AUTH_STATUS];
                        return m ? <Tag color={m.color}>{m.text}</Tag> : conv.shopSummary.authStatus || '—';
                      })()}
                    </Descriptions.Item>
                  </Descriptions>
                ) : (
                  <Typography.Text type="secondary">未关联统一店铺（shops）；可选填便于后续按 shop_id 扩展。 </Typography.Text>
                )}
              </Space>
            </Card>
            <Card size="small" title="关联订单" variant="borderless" style={{ marginBottom: 16 }}>
              <Space direction="vertical" style={{ width: '100%' }}>
                <Space wrap>
                  <Button
                    type="primary"
                    disabled={readOnly}
                    onClick={() => {
                      setOrderPickOpen(true);
                      void runOrderSearch();
                    }}
                  >
                    选择订单
                  </Button>
                  {conv.orderId ? (
                    <Button danger disabled={readOnly} onClick={() => void unlinkOrder()}>
                      取消关联
                    </Button>
                  ) : null}
                </Space>
                {conv.orderSummary ? (
                  <Descriptions bordered size="small" column={{ xs: 1, sm: 2 }}>
                    <Descriptions.Item label="订单号">{conv.orderSummary.orderNo}</Descriptions.Item>
                    <Descriptions.Item label="订单状态">
                      {mapBizStatus(conv.orderSummary.status, ORDER_STATUS)}
                    </Descriptions.Item>
                    <Descriptions.Item label="支付">{mapBizStatus(conv.orderSummary.paymentStatus, ORDER_PAYMENT_STATUS)}</Descriptions.Item>
                    <Descriptions.Item label="履约">{mapBizStatus(conv.orderSummary.fulfillmentStatus, ORDER_FULFILLMENT_STATUS)}</Descriptions.Item>
                    <Descriptions.Item label="订单金额">{`${conv.orderSummary.currency} ${conv.orderSummary.totalAmount}`}</Descriptions.Item>
                    <Descriptions.Item label="SKU 匹配">
                      {mapBizStatus(conv.orderSummary.skuMatchStatus || 'none', ORDER_SKU_MATCH_SUMMARY)}
                    </Descriptions.Item>
                    <Descriptions.Item label="库存扣减">
                      {mapBizStatus(conv.orderSummary.inventoryDeductStatus || 'none', ORDER_INVENTORY_DEDUCT_SUMMARY)}
                    </Descriptions.Item>
                    <Descriptions.Item label="操作" span={2}>
                      <Space wrap>
                        <Button size="small" onClick={() => history.push(`/orders/${conv.orderId}`)}>
                          查看订单详情
                        </Button>
                        <Button size="small" onClick={() => history.push(`/orders/${conv.orderId}?tab=exceptions`)}>
                          查看订单异常
                        </Button>
                        <Button size="small" onClick={() => history.push(`/orders/${conv.orderId}?tab=inventory`)}>
                          查看库存影响
                        </Button>
                      </Space>
                    </Descriptions.Item>
                    <Descriptions.Item label="下单时间">
                      {conv.orderSummary.orderedAt ? formatDateTime(conv.orderSummary.orderedAt) : '—'}
                    </Descriptions.Item>
                    <Descriptions.Item label="最新物流状态" span={2}>
                      {conv.orderSummary.latestShipmentStatus
                        ? mapBizStatus(conv.orderSummary.latestShipmentStatus, ORDER_SHIPMENT_STATUS)
                        : '—'}
                    </Descriptions.Item>
                    {(conv.orderSummary.shipments?.length ?? 0) > 0 ? (
                      <Descriptions.Item label="物流明细" span={2}>
                        <Space direction="vertical" size={4} style={{ width: '100%' }}>
                          {(conv.orderSummary.shipments || []).map((s, i) => (
                            <div key={i}>
                              <Typography.Text>
                                [{mapBizStatus(s.status, ORDER_SHIPMENT_STATUS)}] {s.carrier} · {s.trackingNo || '—'}
                              </Typography.Text>
                              {s.trackingUrl ? (
                                <Typography.Link href={s.trackingUrl} target="_blank" style={{ marginLeft: 8 }}>
                                  追踪
                                </Typography.Link>
                              ) : null}
                            </div>
                          ))}
                        </Space>
                      </Descriptions.Item>
                    ) : null}
                  </Descriptions>
                ) : (
                  <Typography.Text type="secondary">未关联订单；生成建议时将缺少订单、商品、库存等上下文。</Typography.Text>
                )}
              </Space>
            </Card>
            {(conv.productContexts?.length ?? 0) > 0 ? (
              <Card size="small" title="关联商品" variant="borderless" style={{ marginBottom: 16 }}>
                <List
                  size="small"
                  dataSource={conv.productContexts}
                  renderItem={(p) => (
                    <List.Item>
                      <Space direction="vertical" size={0}>
                        <Typography.Text strong>{p.productTitle || '—'}</Typography.Text>
                        <Typography.Text type="secondary">
                          SKU: {p.skuCode || '—'} · {p.skuName || '—'} · 库存: {p.stockStatus || '—'}
                        </Typography.Text>
                        {p.productId ? (
                          <Space>
                            <Typography.Link onClick={() => history.push(`/product/drafts/${p.productId}`)}>
                              查看商品
                            </Typography.Link>
                            <Typography.Link onClick={() => history.push(`/product/drafts/${p.productId}?tab=inventory`)}>
                              查看库存
                            </Typography.Link>
                          </Space>
                        ) : null}
                      </Space>
                    </List.Item>
                  )}
                />
              </Card>
            ) : null}
            {(conv.inventoryContexts?.length ?? 0) > 0 ? (
              <Card size="small" title="库存 / SKU 上下文" variant="borderless" style={{ marginBottom: 16 }}>
                <List
                  size="small"
                  dataSource={conv.inventoryContexts}
                  renderItem={(p) => (
                    <List.Item>
                      {p.skuCode} · {p.skuName} · 库存 {p.stock ?? '—'} · {p.stockStatus} · 绑定 {p.bindStatus}
                    </List.Item>
                  )}
                />
              </Card>
            ) : null}
            {conv.contextSummary ? (
              <Card size="small" title="AI 上下文摘要" variant="borderless" style={{ marginBottom: 16 }}>
                <Descriptions size="small" column={{ xs: 1, sm: 2 }}>
                  <Descriptions.Item label="订单状态">{conv.contextSummary.orderStatus || '—'}</Descriptions.Item>
                  <Descriptions.Item label="SKU 匹配">{conv.contextSummary.skuMatchStatus || '—'}</Descriptions.Item>
                  <Descriptions.Item label="库存状态">{conv.contextSummary.inventoryStatus || '—'}</Descriptions.Item>
                  <Descriptions.Item label="商品">{conv.contextSummary.productTitle || '—'}</Descriptions.Item>
                  <Descriptions.Item label="客户问题" span={2}>
                    {conv.contextSummary.customerQuestion || '—'}
                  </Descriptions.Item>
                </Descriptions>
              </Card>
            ) : null}
          </>
        )}

        <Row gutter={[16, 16]}>
          <Col xs={24} lg={14}>
            <Card title="消息时间线" variant="borderless">
              {msgs.length === 0 ? (
                <Empty description="暂无消息" />
              ) : (
                <List
                  itemLayout="vertical"
                  dataSource={msgs}
                  renderItem={(item) => {
                    const isCustomer = item.role === 'customer';
                    return (
                      <List.Item style={{ borderBlockEnd: 'none' }}>
                        <div
                          style={{
                            display: 'flex',
                            justifyContent: isCustomer ? 'flex-start' : 'flex-end',
                          }}
                        >
                          <Card
                            size="small"
                            style={{
                              maxWidth: '92%',
                              background: isCustomer ? 'var(--ant-color-fill-quaternary, #fafafa)' : '#e6f4ff',
                            }}
                            title={
                              <Space size={8}>
                                <Typography.Text type="secondary">
                                  {formatDateTime(item.createdAt)}
                                </Typography.Text>
                                <Tag>{CUSTOMER_MESSAGE_ROLE_LABEL[item.role] ?? item.role}</Tag>
                                <Tag>{CUSTOMER_MESSAGE_SOURCE_LABEL[item.source] ?? item.source}</Tag>
                                {item.messageType ? (
                                  <Tag>{CUSTOMER_MESSAGE_TYPE_LABEL[item.messageType] ?? item.messageType}</Tag>
                                ) : null}
                              </Space>
                            }
                          >
                            <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
                              {item.content}
                            </Typography.Paragraph>
                          </Card>
                        </div>
                      </List.Item>
                    );
                  }}
                />
              )}
              <div style={{ marginTop: 16 }}>
                <Typography.Text strong>添加客户消息</Typography.Text>
                <Input.TextArea
                  rows={3}
                  value={newCustomerMsg}
                  onChange={(e) => setNewCustomerMsg(e.target.value)}
                  placeholder="录入客户原话…"
                  style={{ marginTop: 8 }}
                  disabled={readOnly}
                />
                <Button
                  type="primary"
                  style={{ marginTop: 8 }}
                  disabled={readOnly}
                  onClick={() => void onAddCustomerMessage()}
                >
                  添加客户消息
                </Button>
              </div>
            </Card>
          </Col>

          <Col xs={24} lg={10}>
            <Card title="AI 回复建议" variant="borderless">
              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                <Alert
                  type="warning"
                  showIcon
                  message="人工审核"
                  description="AI 回复仅为建议；涉及退款、赔偿、投诉、履约承诺等事项请务必人工确认，勿直接对外生效。"
                  style={{ width: '100%' }}
                />
                <div>
                  <Typography.Text type="secondary">针对客户消息（可选，默认取最近一条 customer）</Typography.Text>
                  <Select
                    allowClear
                    placeholder="选择客户消息"
                    style={{ width: '100%', marginTop: 4 }}
                    options={customerMessageOptions}
                    value={focusMessageId}
                    onChange={(v) => setFocusMessageId(v)}
                  />
                </div>
                <Space wrap>
                  <span>language</span>
                  <Input style={{ width: 100 }} value={lang} onChange={(e) => setLang(e.target.value)} />
                  <span>tone</span>
                  <Input style={{ width: 140 }} value={tone} onChange={(e) => setTone(e.target.value)} />
                </Space>
                <div>
                  <Typography.Text>platform</Typography.Text>
                  <Input
                    style={{ marginTop: 4 }}
                    value={platform}
                    onChange={(e) => setPlatform(e.target.value)}
                    placeholder="manual"
                  />
                </div>
                <div>
                  <Typography.Text>shopPolicy（可选）</Typography.Text>
                  <Input.TextArea
                    rows={3}
                    value={shopPolicy}
                    onChange={(e) => setShopPolicy(e.target.value)}
                    style={{ marginTop: 4 }}
                    placeholder="店铺政策摘要，可为空"
                  />
                </div>
                <Button type="primary" loading={genLoading} disabled={readOnly} onClick={() => void onGenerate()}>
                  AI 生成建议回复
                </Button>
                <Typography.Paragraph type="secondary" style={{ marginBottom: 0, fontSize: 12 }}>
                  说明：生成内容需人工确认；系统不会自动向任何外部平台发送消息。
                </Typography.Paragraph>

                {aiMeta && (
                  <Descriptions size="small" column={1} bordered>
                    <Descriptions.Item label="买家意图">{aiMeta.intent || '—'}</Descriptions.Item>
                    <Descriptions.Item label="情绪倾向">{aiMeta.sentiment || '—'}</Descriptions.Item>
                    <Descriptions.Item label="风险等级">{riskTag(aiMeta.riskLevel)}</Descriptions.Item>
                    <Descriptions.Item label="备注">{aiMeta.notes || '—'}</Descriptions.Item>
                  </Descriptions>
                )}
                <div>
                  <Typography.Text strong>回复内容</Typography.Text>
                  <Typography.Paragraph type="secondary" style={{ marginTop: 4, marginBottom: 8, fontSize: 12 }}>
                    可先用 AI 生成并编辑，或直接手写后再「采纳为内部回复」或「发送到平台」（均须人工操作）。
                  </Typography.Paragraph>
                  <Input.TextArea
                    rows={6}
                    value={editedReply}
                    onChange={(e) => setEditedReply(e.target.value)}
                    placeholder="编辑或手写回复内容…"
                  />
                  <Space style={{ marginTop: 8 }} wrap>
                    <Button disabled={readOnly} onClick={() => void openTemplatePicker()}>
                      插入话术模板
                    </Button>
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      模板插入后仍可编辑，发送需人工确认
                    </Typography.Text>
                  </Space>
                </div>
                <Space wrap align="start">
                  <Button onClick={() => void onSaveEdit()} disabled={readOnly || !suggestionId}>
                    保存编辑
                  </Button>
                  <Button type="primary" onClick={() => void onAccept()} disabled={readOnly || !suggestionId}>
                    采纳为内部回复
                  </Button>
                  <Button
                    type="primary"
                    ghost
                    onClick={() => void onSendToPlatform()}
                    disabled={readOnly || !canSendToPlatform || !editedReply.trim()}
                  >
                    发送到平台
                  </Button>
                  <Button danger onClick={() => void onDiscard()} disabled={readOnly || !suggestionId}>
                    废弃建议
                  </Button>
                  <Button onClick={() => void onCopy()}>复制回复</Button>
                </Space>
                {!canSendToPlatform ? (
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    「发送到平台」需本会话已关联店铺，且存在平台外部会话 ID（通常由「拉取平台消息」写入）。手工录入会话可无平台外发。
                  </Typography.Text>
                ) : null}
              </Space>
            </Card>
          </Col>
        </Row>

        <Modal
          title="选择关联订单（手工录入的订单）"
          open={orderPickOpen}
          onCancel={() => setOrderPickOpen(false)}
          footer={null}
          width={680}
          destroyOnHidden
        >
          <Space wrap style={{ marginBottom: 12 }}>
            <Input
              placeholder="订单号筛选"
              value={orderFilterNo}
              onChange={(e) => setOrderFilterNo(e.target.value)}
              style={{ width: 180 }}
              allowClear
            />
            <Input
              placeholder="客户姓名"
              value={orderFilterName}
              onChange={(e) => setOrderFilterName(e.target.value)}
              style={{ width: 160 }}
              allowClear
            />
            <Button loading={orderSearchLoading} onClick={() => void runOrderSearch()}>
              查询
            </Button>
          </Space>
          <List<OrderListRow>
            dataSource={orderHits}
            loading={orderSearchLoading}
            locale={{ emptyText: '暂无数据，可先输入条件再查询' }}
            renderItem={(row) => (
              <List.Item
                actions={[
                  <a key="lnk" onClick={() => void linkOrder(row.id)}>
                    关联
                  </a>,
                ]}
              >
                <List.Item.Meta
                  title={`${row.orderNo} · ${row.customerName}`}
                  description={
                    <Space wrap size={8}>
                      {mapBizStatus(row.status, ORDER_STATUS)}
                      {mapBizStatus(row.paymentStatus, ORDER_PAYMENT_STATUS)}
                      {mapBizStatus(row.fulfillmentStatus, ORDER_FULFILLMENT_STATUS)}
                      <Typography.Text type="secondary">
                        {row.currency} {row.totalAmount}
                      </Typography.Text>
                    </Space>
                  }
                />
              </List.Item>
            )}
          />
        </Modal>

        <Modal
          title="插入话术模板"
          open={tplOpen}
          onCancel={() => setTplOpen(false)}
          footer={null}
          width={680}
          destroyOnHidden
        >
          <Tabs
            activeKey={tplGroup}
            onChange={(k) => setTplGroup(k as 'all' | ReplyTemplateGroupKey)}
            items={[
              { key: 'all', label: '全部' },
              ...REPLY_TEMPLATE_GROUPS.map((g) => ({ key: g.key, label: g.label })),
            ]}
          />
          <Input.Search
            allowClear
            placeholder="按名称 / 内容搜索话术"
            style={{ marginBottom: 12 }}
            value={tplKeyword}
            onChange={(e) => setTplKeyword(e.target.value)}
          />
          <List<ReplyTemplateRow>
            dataSource={visibleTemplates}
            loading={tplLoading}
            locale={{ emptyText: '没有可用话术模板，可到「客服 → 话术模板」页面新增' }}
            style={{ maxHeight: 360, overflowY: 'auto' }}
            renderItem={(tpl) => (
              <List.Item
                actions={[
                  <Button key="ins" size="small" type="link" onClick={() => insertTemplate(tpl)}>
                    插入
                  </Button>,
                ]}
              >
                <List.Item.Meta
                  title={
                    <Space size={8}>
                      <Typography.Text strong>{tpl.name}</Typography.Text>
                      <Tag>{replyTemplateGroupLabel(tpl.groupKey)}</Tag>
                    </Space>
                  }
                  description={
                    <Typography.Paragraph
                      type="secondary"
                      style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}
                      ellipsis={{ rows: 2, tooltip: tpl.content }}
                    >
                      {tpl.content}
                    </Typography.Paragraph>
                  }
                />
              </List.Item>
            )}
          />
        </Modal>

        <Modal
          title="选择关联店铺"
          open={shopPickOpen}
          onCancel={() => setShopPickOpen(false)}
          onOk={() => void linkShop()}
          okText="关联"
          cancelText="取消"
          destroyOnHidden
        >
          <Select
            showSearch
            optionFilterProp="label"
            placeholder="选择店铺（shops 统一表）"
            style={{ width: '100%' }}
            options={shopOpts}
            value={pickedShopId}
            onChange={(v) => setPickedShopId(v)}
            allowClear
          />
        </Modal>
      </Spin>
    </TmPageContainer>
  );
}
