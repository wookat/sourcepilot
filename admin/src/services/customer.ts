import { deleteJSON, getJSON, getWithParams, postJSON, putJSON } from '@/services/request';
import type { ReplyTemplateGroupKey } from '@/utils/replyTemplateVars';

export type { ReplyTemplateGroupKey } from '@/utils/replyTemplateVars';

export type ConversationRow = {
  id: string;
  platform: string;
  shopId?: string;
  shopName?: string;
  shopPlatform?: string;
  customerName: string;
  customerNameMasked?: string;
  customerLanguage: string;
  status: string;
  lastMessageAt?: string;
  createdAt: string;
  updatedAt: string;
  messageCount: number;
  latestMessage?: string;
  orderId?: string;
  orderNo?: string;
  productTitle?: string;
  aiSuggestionStatus?: string;
  sendStatus?: string;
  openFailureCount?: number;
};

export type ConversationShopSummary = {
  id: string;
  platform: string;
  shopName: string;
  shopCode?: string;
  status: string;
  authStatus: string;
};

export type ConversationOrderShipment = {
  carrier: string;
  trackingNo: string;
  trackingUrl?: string;
  status: string;
  shippedAt?: string;
  deliveredAt?: string;
};

export type ConversationOrderSummary = {
  id: string;
  orderNo: string;
  platform: string;
  status: string;
  paymentStatus: string;
  fulfillmentStatus: string;
  currency: string;
  totalAmount: number;
  itemCount?: number;
  skuMatchStatus?: string;
  inventoryDeductStatus?: string;
  orderedAt?: string;
  latestShipmentStatus?: string;
  shipments?: ConversationOrderShipment[];
};

export type ConversationDetail = {
  id: string;
  platform: string;
  shopId?: string;
  externalConversationId?: string;
  customerName: string;
  customerNameMasked?: string;
  customerAvatar?: string;
  customerLanguage: string;
  status: string;
  lastMessageAt?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
  orderId?: string;
  orderSummary?: ConversationOrderSummary | null;
  shopSummary?: ConversationShopSummary | null;
  productContexts?: ProductContextItem[];
  inventoryContexts?: InventoryContextItem[];
  contextSummary?: ContextSummary | null;
  openFailureCount?: number;
  canWrite?: boolean;
};

export type ContextSummary = {
  orderStatus?: string;
  skuMatchStatus?: string;
  inventoryStatus?: string;
  productTitle?: string;
  customerQuestion?: string;
  incompleteWarning?: string;
};

export type ProductContextItem = {
  productId?: string;
  productTitle?: string;
  skuCode?: string;
  skuName?: string;
  stockStatus?: string;
  publishStatus?: string;
  aiOpsStatus?: string;
};

export type InventoryContextItem = {
  skuCode?: string;
  skuName?: string;
  stock?: number;
  stockStatus?: string;
  bindStatus?: string;
};

export type CustomerDashboardSummary = {
  pendingReplyCount: number;
  todayNewMessages: number;
  aiSuggestionPendingCount: number;
  sendFailureCount: number;
  unauthorizedShopCount: number;
  syncTaskFailureCount: number;
  openConversationCount: number;
};

export type SuggestionRow = {
  id: string;
  conversationId: string;
  messageId?: string;
  status: string;
  suggestedReply?: string;
  editedReply?: string;
  rejectReason?: string;
  language?: string;
  tone?: string;
  contextSummary?: ContextSummary;
  createdAt: string;
  updatedAt: string;
};

export type CustomerMessageRow = {
  id: string;
  conversationId: string;
  role: string;
  content: string;
  language: string;
  messageType?: string;
  source: string;
  externalMessageId?: string;
  rawData?: unknown;
  createdBy?: string;
  createdAt: string;
};

export type GenerateReplyResult = {
  suggestionId: string;
  reply: string;
  intent: string;
  sentiment: string;
  riskLevel: string;
  notes: string;
  taskId: string;
  contextSummary?: ContextSummary;
};

type Paginated<T> = {
  list: T[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
};

function boolQueryFlag(v: boolean | string | undefined): string | undefined {
  if (v === true || v === 'true' || v === '1') return '1';
  if (v === false || v === 'false' || v === '0') return '0';
  return undefined;
}

export async function queryConversations(params: {
  page?: number;
  pageSize?: number;
  platform?: string;
  status?: string;
  shopId?: string;
  customerName?: string;
  keyword?: string;
  pendingReply?: boolean | string;
  hasAiSuggestion?: boolean | string;
  sendFailed?: boolean | string;
  hasOrder?: boolean | string;
  start?: string;
  end?: string;
  updatedStart?: string;
  updatedEnd?: string;
}): Promise<Paginated<ConversationRow>> {
  return getWithParams('/api/v1/customer/conversations', {
    page: params.page,
    pageSize: params.pageSize,
    platform: params.platform,
    status: params.status,
    shopId: params.shopId,
    customerName: params.customerName,
    keyword: params.keyword,
    pendingReply: boolQueryFlag(params.pendingReply),
    hasAiSuggestion: boolQueryFlag(params.hasAiSuggestion),
    sendFailed: boolQueryFlag(params.sendFailed),
    hasOrder: boolQueryFlag(params.hasOrder),
    start: params.start,
    end: params.end,
    updatedStart: params.updatedStart,
    updatedEnd: params.updatedEnd,
  });
}

export async function getCustomerDashboard(): Promise<CustomerDashboardSummary> {
  return getJSON('/api/v1/customer/dashboard');
}

export async function querySuggestions(conversationId: string): Promise<{ list: SuggestionRow[] }> {
  return getJSON(`/api/v1/customer/conversations/${conversationId}/ai-suggestions`);
}

export async function rejectReplySuggestion(id: string, payload: { reason?: string }): Promise<{ ok: boolean }> {
  return postJSON(`/api/v1/customer/ai-suggestions/${id}/reject`, payload);
}

export async function createConversation(payload: {
  platform?: string;
  shopId?: string;
  customerName: string;
  customerLanguage?: string;
  customerAvatar?: string;
}): Promise<ConversationDetail> {
  return postJSON('/api/v1/customer/conversations', payload);
}

export async function getConversation(id: string): Promise<ConversationDetail> {
  return getJSON(`/api/v1/customer/conversations/${id}`);
}

export async function updateConversation(
  id: string,
  payload: {
    customerName?: string;
    customerLanguage?: string;
    status?: string;
    shopId?: string;
    orderId?: string;
  },
): Promise<ConversationDetail> {
  return putJSON(`/api/v1/customer/conversations/${id}`, payload);
}

export async function deleteConversation(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/customer/conversations/${id}`);
}

export async function queryMessages(conversationId: string): Promise<{ list: CustomerMessageRow[] }> {
  return getJSON(`/api/v1/customer/conversations/${conversationId}/messages`);
}

export async function createMessage(
  conversationId: string,
  payload: { role: string; content: string; language?: string; source?: string },
): Promise<CustomerMessageRow> {
  return postJSON(`/api/v1/customer/conversations/${conversationId}/messages`, payload);
}

export async function markConversationReplied(conversationId: string, reply: string): Promise<CustomerMessageRow> {
  return postJSON(`/api/v1/customer/conversations/${conversationId}/mark-replied`, { reply });
}

export async function generateCustomerReply(
  conversationId: string,
  payload: {
    messageId?: string;
    language?: string;
    tone?: string;
    platform?: string;
    shopPolicy?: string;
  },
): Promise<GenerateReplyResult> {
  return postJSON(`/api/v1/customer/conversations/${conversationId}/ai/generate-reply`, payload);
}

export async function updateReplySuggestion(
  id: string,
  payload: { editedReply: string },
): Promise<{ ok: boolean }> {
  return putJSON(`/api/v1/customer/reply-suggestions/${id}`, payload);
}

export async function acceptReplySuggestion(
  id: string,
  payload: { finalReply: string },
): Promise<{ ok: boolean }> {
  return postJSON(`/api/v1/customer/reply-suggestions/${id}/accept`, payload);
}

export async function discardReplySuggestion(id: string): Promise<{ ok: boolean }> {
  return postJSON(`/api/v1/customer/reply-suggestions/${id}/discard`, {});
}

export type CustomerMessageSyncTaskRow = {
  id: string;
  shopId: string;
  shopName?: string;
  platform: string;
  taskType: string;
  status: string;
  mode: string;
  cursor?: string;
  startedAt?: string;
  finishedAt?: string;
  totalCount: number;
  successCount: number;
  failedCount: number;
  errorMessage?: string;
  input?: unknown;
  output?: unknown;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export async function syncCustomerMessages(
  shopId: string,
  payload: { mode?: string; start?: string; end?: string; cursor?: string; limit?: number },
): Promise<CustomerMessageSyncTaskRow> {
  return postJSON(`/api/v1/shops/${shopId}/sync-customer-messages`, payload);
}

export async function queryCustomerMessageSyncTasks(params: {
  page?: number;
  pageSize?: number;
  shopId?: string;
  platform?: string;
  status?: string;
  start?: string;
  end?: string;
}): Promise<{ list: CustomerMessageSyncTaskRow[]; pagination: { page: number; pageSize: number; total: number; totalPages: number } }> {
  return getWithParams('/api/v1/customer/message-sync/tasks', {
    page: params.page,
    pageSize: params.pageSize,
    shopId: params.shopId,
    platform: params.platform,
    status: params.status,
    start: params.start,
    end: params.end,
  });
}

export async function getCustomerMessageSyncTask(id: string): Promise<CustomerMessageSyncTaskRow> {
  return getJSON(`/api/v1/customer/message-sync/tasks/${id}`);
}

export async function retryCustomerMessageSyncTask(id: string): Promise<CustomerMessageSyncTaskRow> {
  return postJSON(`/api/v1/customer/message-sync/tasks/${id}/retry`, {});
}

export async function sendPlatformMessage(
  conversationId: string,
  payload: { reply: string; suggestionId?: string; idempotencyKey?: string },
): Promise<CustomerMessageRow> {
  return postJSON(`/api/v1/customer/conversations/${conversationId}/send-platform-message`, payload);
}

// ---- 客服话术模板 ----

/** 模板语言表（与后端 customerchat.TemplateLanguages 保持一致，可扩展） */
export const TEMPLATE_LANGUAGES: { key: string; label: string }[] = [
  { key: 'zh-CN', label: '中文（简体）' },
  { key: 'en', label: '英语' },
  { key: 'es', label: '西班牙语' },
  { key: 'pt', label: '葡萄牙语' },
  { key: 'fr', label: '法语' },
  { key: 'de', label: '德语' },
  { key: 'it', label: '意大利语' },
  { key: 'ru', label: '俄语' },
  { key: 'ja', label: '日语' },
  { key: 'ko', label: '韩语' },
  { key: 'th', label: '泰语' },
  { key: 'vi', label: '越南语' },
  { key: 'id', label: '印尼语' },
  { key: 'ms', label: '马来语' },
  { key: 'ar', label: '阿拉伯语' },
];

export function templateLanguageLabel(lang: string): string {
  return TEMPLATE_LANGUAGES.find((l) => l.key === lang)?.label || lang;
}

export type ReplyTemplateVariant = {
  language: string;
  content: string;
};

export type ReplyTemplateRow = {
  id: string;
  groupKey: ReplyTemplateGroupKey;
  name: string;
  content: string;
  sortOrder: number;
  enabled: boolean;
  /** 默认语言（content 字段所用语言） */
  defaultLanguage: string;
  /** 其他语言变体（不含默认语言） */
  variants: ReplyTemplateVariant[];
  createdAt: string;
  updatedAt: string;
};

export type ReplyTemplateListQuery = {
  group?: ReplyTemplateGroupKey;
  keyword?: string;
  enabled?: boolean;
};

export async function queryReplyTemplates(
  query: ReplyTemplateListQuery = {},
): Promise<{ list: ReplyTemplateRow[]; canWrite: boolean }> {
  return getWithParams('/api/v1/customer/reply-templates', {
    group: query.group,
    keyword: query.keyword,
    enabled: query.enabled === undefined ? undefined : query.enabled ? 'true' : 'false',
  });
}

export type ReplyTemplateUpsertBody = {
  groupKey?: ReplyTemplateGroupKey;
  name?: string;
  content?: string;
  sortOrder?: number;
  enabled?: boolean;
  defaultLanguage?: string;
  /** 非 undefined 时全量替换语言变体 */
  variants?: ReplyTemplateVariant[];
};

export async function createReplyTemplate(body: ReplyTemplateUpsertBody): Promise<ReplyTemplateRow> {
  return postJSON('/api/v1/customer/reply-templates', body);
}

export async function updateReplyTemplate(
  id: string,
  body: ReplyTemplateUpsertBody,
): Promise<ReplyTemplateRow> {
  return putJSON(`/api/v1/customer/reply-templates/${id}`, body);
}

export async function deleteReplyTemplate(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/customer/reply-templates/${id}`);
}

export async function reorderReplyTemplates(payload: {
  groupKey: ReplyTemplateGroupKey;
  ids: string[];
}): Promise<{ ok: boolean }> {
  return postJSON('/api/v1/customer/reply-templates/reorder', payload);
}

// ---- 买家自动消息（节点规则 + 待发草稿，站内人工确认闭环，绝不自动外发） ----

export type BuyerMsgNode = 'paid' | 'shipped' | 'delivered' | 'logistics_exception' | 'refunded';

export const BUYER_MSG_NODES: { key: BuyerMsgNode; label: string }[] = [
  { key: 'paid', label: '已付款' },
  { key: 'shipped', label: '已发货' },
  { key: 'delivered', label: '已签收' },
  { key: 'logistics_exception', label: '物流异常' },
  { key: 'refunded', label: '退款' },
];

export function buyerMsgNodeLabel(node: string): string {
  return BUYER_MSG_NODES.find((n) => n.key === node)?.label || node;
}

export type BuyerMsgDraftStatus = 'pending' | 'sent' | 'ignored';

export const BUYER_MSG_DRAFT_STATUSES: { key: BuyerMsgDraftStatus; label: string }[] = [
  { key: 'pending', label: '待发送' },
  { key: 'sent', label: '已发送' },
  { key: 'ignored', label: '已忽略' },
];

export function buyerMsgDraftStatusLabel(status: string): string {
  return BUYER_MSG_DRAFT_STATUSES.find((s) => s.key === status)?.label || status;
}

export type BuyerMsgRuleRow = {
  id: string;
  name: string;
  node: BuyerMsgNode;
  templateId: string;
  templateName: string;
  templateMissing?: boolean;
  enabled: boolean;
  /** 为空表示回溯存量；非空时仅对该时刻后的订单事件生成草稿 */
  effectiveFrom?: string;
  /** true 表示已开启「回溯存量订单」 */
  backfill?: boolean;
  platforms: string[];
  shopIds: string[];
};

export type BuyerMsgRuleBody = {
  name?: string;
  node?: BuyerMsgNode;
  templateId?: string;
  enabled?: boolean;
  platforms?: string[];
  shopIds?: string[];
  /** 显式开启回溯存量订单（默认不回溯，仅对规则生效后的新订单事件生成草稿） */
  backfill?: boolean;
};

export async function queryBuyerMsgRules(): Promise<{ list: BuyerMsgRuleRow[]; canWrite: boolean }> {
  return getJSON('/api/v1/customer/buyer-message-rules');
}

export async function createBuyerMsgRule(body: BuyerMsgRuleBody): Promise<BuyerMsgRuleRow> {
  return postJSON('/api/v1/customer/buyer-message-rules', body);
}

export async function updateBuyerMsgRule(id: string, body: BuyerMsgRuleBody): Promise<BuyerMsgRuleRow> {
  return putJSON(`/api/v1/customer/buyer-message-rules/${id}`, body);
}

export async function deleteBuyerMsgRule(id: string): Promise<{ ok: boolean }> {
  return deleteJSON(`/api/v1/customer/buyer-message-rules/${id}`);
}

/** 「回溯存量」开启时将生成的草稿数量预估（只读，不产生草稿） */
export async function estimateBuyerMsgBackfill(params: {
  node: BuyerMsgNode;
  platforms?: string[];
  shopIds?: string[];
}): Promise<{ estimated: number }> {
  const qs = new URLSearchParams({ node: params.node });
  if (params.platforms?.length) qs.set('platforms', params.platforms.join(','));
  if (params.shopIds?.length) qs.set('shopIds', params.shopIds.join(','));
  return getJSON(`/api/v1/customer/buyer-message-rules/backfill-estimate?${qs.toString()}`);
}

export type BuyerMsgDraftRow = {
  id: string;
  orderId: string;
  orderNo: string;
  customerName: string;
  node: BuyerMsgNode;
  ruleId: string;
  templateId: string;
  templateName: string;
  platform: string;
  shopId?: string;
  shopName?: string;
  content: string;
  /** 草稿内容所用模板语言 */
  language: string;
  /** 目标语言来源：order_country / shop_language / platform / fallback / no_variant / manual */
  langSource: string;
  missingVars: string[];
  status: BuyerMsgDraftStatus;
  conversationId?: string;
  sentAt?: string;
  ignoredAt?: string;
  createdAt: string;
  updatedAt: string;
};

export async function queryBuyerMsgDrafts(params: {
  page?: number;
  pageSize?: number;
  node?: string;
  status?: string;
  platform?: string;
  shopId?: string;
  keyword?: string;
}): Promise<{
  list: BuyerMsgDraftRow[];
  total: number;
  page: number;
  pageSize: number;
  canWrite: boolean;
}> {
  return getWithParams('/api/v1/customer/buyer-messages/drafts', {
    page: params.page,
    pageSize: params.pageSize,
    node: params.node,
    status: params.status,
    platform: params.platform,
    shopId: params.shopId,
    keyword: params.keyword,
  });
}

export async function generateBuyerMsgDrafts(): Promise<{ created: number }> {
  return postJSON('/api/v1/customer/buyer-messages/generate', {});
}

export async function updateBuyerMsgDraft(id: string, content: string): Promise<BuyerMsgDraftRow> {
  return putJSON(`/api/v1/customer/buyer-messages/drafts/${id}`, { content });
}

/** 按所选语言变体重新生成草稿内容（只改草稿，不发送任何平台消息） */
export async function regenerateBuyerMsgDraft(id: string, language: string): Promise<BuyerMsgDraftRow> {
  return postJSON(`/api/v1/customer/buyer-messages/drafts/${id}/regenerate`, { language });
}

export async function markBuyerMsgDraftSent(id: string): Promise<BuyerMsgDraftRow> {
  return postJSON(`/api/v1/customer/buyer-messages/drafts/${id}/mark-sent`, {});
}

export async function ignoreBuyerMsgDraft(id: string): Promise<BuyerMsgDraftRow> {
  return postJSON(`/api/v1/customer/buyer-messages/drafts/${id}/ignore`, {});
}

export async function batchMarkBuyerMsgDraftsSent(ids: string[]): Promise<{ updated: number; skipped: number }> {
  return postJSON('/api/v1/customer/buyer-messages/drafts/batch-mark-sent', { ids });
}
