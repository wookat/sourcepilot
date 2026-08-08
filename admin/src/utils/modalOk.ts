import { message } from 'antd';

/**
 * 构造静态 Modal.confirm 的非 async onOk：由回调手动 close，
 * 成功才关闭弹窗；失败 toast 中文提示、弹窗保持打开，且不向外
 * 抛拒绝（避免 dev 环境 unhandledrejection 触发 react-error-overlay）。
 */
export function modalOk(run: () => void | Promise<void>, errorText?: (e: unknown) => string) {
  return (close: () => void) => {
    Promise.resolve()
      .then(() => run())
      .then(() => close())
      .catch((e: unknown) => {
        message.error(errorText ? errorText(e) : (e as Error)?.message || '操作失败');
      });
  };
}
