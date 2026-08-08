import { Modal, message } from 'antd';
import type { ModalFuncProps } from 'antd/es/modal/interface';

export type SensitiveConfirmOptions = {
  title: string;
  content: string;
  impacts?: string[];
  externalCall?: boolean;
  reversible?: boolean;
  failureHint?: string;
  onOk: () => void | Promise<void>;
  /** Custom failure toast text; defaults to the error message. */
  errorText?: (e: unknown) => string;
};

/** Standard sensitive-operation confirm dialog (F5 audit UX). */
export function confirmSensitiveAction(opts: SensitiveConfirmOptions) {
  const lines = [opts.content];
  if (opts.impacts?.length) {
    lines.push('', '将影响：', ...opts.impacts.map((x) => `· ${x}`));
  }
  if (opts.externalCall) {
    lines.push('', '此操作可能调用外部平台接口。');
  }
  lines.push('', opts.reversible === false ? '此操作通常不可撤销。' : '部分操作可在任务中心查看失败并重试。');
  if (opts.failureHint) {
    lines.push(`失败后可在：${opts.failureHint}`);
  }
  let pending = false;
  const modalOpts: ModalFuncProps = {
    title: opts.title,
    content: <div style={{ whiteSpace: 'pre-wrap' }}>{lines.join('\n')}</div>,
    okText: '确认执行',
    cancelText: '取消',
    // onOk 不返回 Promise，由回调手动 close：成功才关闭，失败 toast 且
    // 弹窗保持打开、不向外抛拒绝（避免 dev 环境 unhandledrejection 触发
    // react-error-overlay 盖住 toast）。
    onOk: (close: () => void) => {
      if (pending) return;
      pending = true;
      Promise.resolve()
        .then(() => opts.onOk())
        .then(() => close())
        .catch((e) => {
          message.error(opts.errorText ? opts.errorText(e) : (e as Error)?.message || '操作失败');
        })
        .finally(() => {
          pending = false;
        });
    },
  };
  Modal.confirm(modalOpts);
}
