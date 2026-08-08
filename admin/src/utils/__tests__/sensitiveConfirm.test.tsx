import { describe, expect, it, vi, beforeEach } from 'vitest';
import type { ModalFuncProps } from 'antd';

vi.mock('antd', () => ({
  message: { error: vi.fn(), success: vi.fn() },
  Modal: { confirm: vi.fn() },
}));

import { message, Modal } from 'antd';
import { confirmSensitiveAction } from '../sensitiveConfirm';

const flush = () => new Promise((r) => setTimeout(r, 0));

function lastConfirmOpts(): ModalFuncProps {
  const calls = (Modal.confirm as ReturnType<typeof vi.fn>).mock.calls;
  return calls[calls.length - 1][0] as ModalFuncProps;
}

describe('confirmSensitiveAction', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('onOk 声明 close 参数，成功后才关闭', async () => {
    const done = vi.fn();
    confirmSensitiveAction({ title: 't', content: 'c', onOk: done });
    const onOk = lastConfirmOpts().onOk as (close: () => void) => void;
    expect(onOk.length).toBe(1);
    const close = vi.fn();
    onOk(close);
    await flush();
    expect(done).toHaveBeenCalled();
    expect(close).toHaveBeenCalledTimes(1);
  });

  it('失败时弹窗保持打开并 toast，支持自定义 errorText', async () => {
    confirmSensitiveAction({
      title: 't',
      content: 'c',
      onOk: async () => {
        throw new Error('raw');
      },
      errorText: () => '中文失败提示',
    });
    const onOk = lastConfirmOpts().onOk as (close: () => void) => void;
    const close = vi.fn();
    onOk(close);
    await flush();
    expect(close).not.toHaveBeenCalled();
    expect(message.error).toHaveBeenCalledWith('中文失败提示');
  });
});
