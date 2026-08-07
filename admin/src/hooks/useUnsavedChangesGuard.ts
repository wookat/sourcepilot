import { history } from '@umijs/max';
import { Modal } from 'antd';
import { useEffect } from 'react';

/** history v5 的 block 在运行时存在，但 Umi 生成的 UmiHistory 类型未声明。 */
type BlockableHistory = {
  block: (blocker: (tx: { retry: () => void }) => void) => () => void;
};

/**
 * 页面存在未保存更改时，拦截路由跳转与浏览器关闭/刷新，弹出确认后才允许离开。
 */
export function useUnsavedChangesGuard(
  dirty: boolean,
  content = '当前页面有未保存的更改，离开后修改将丢失。',
) {
  useEffect(() => {
    if (!dirty) return undefined;
    const unblock = (history as unknown as BlockableHistory).block((tx) => {
      Modal.confirm({
        title: '离开当前页面？',
        content,
        okText: '离开',
        okButtonProps: { danger: true },
        cancelText: '继续编辑',
        onOk: () => {
          unblock();
          tx.retry();
        },
      });
    });
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = '';
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => {
      unblock();
      window.removeEventListener('beforeunload', onBeforeUnload);
    };
  }, [dirty, content]);
}
