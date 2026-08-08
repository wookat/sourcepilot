import { describe, expect, it, vi, beforeEach } from 'vitest';

vi.mock('antd', () => ({
  message: { error: vi.fn(), success: vi.fn() },
  Modal: { confirm: vi.fn() },
}));

import { message } from 'antd';
import { modalOk } from '../modalOk';

const flush = () => new Promise((r) => setTimeout(r, 0));

describe('modalOk', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('声明 close 参数（antd 不会在返回前自动关闭弹窗）', () => {
    expect(modalOk(async () => {}).length).toBe(1);
  });

  it('成功后才调用 close', async () => {
    const close = vi.fn();
    modalOk(async () => {})(close);
    await flush();
    expect(close).toHaveBeenCalledTimes(1);
    expect(message.error).not.toHaveBeenCalled();
  });

  it('失败时不 close、toast 中文提示且不向外抛拒绝', async () => {
    const close = vi.fn();
    modalOk(async () => {
      throw new Error('后端拒绝');
    })(close);
    await flush();
    expect(close).not.toHaveBeenCalled();
    expect(message.error).toHaveBeenCalledWith('后端拒绝');
  });

  it('提交中重复触发不产生第二次执行（防重复提交）', async () => {
    const run = vi.fn(() => new Promise<void>((r) => setTimeout(r, 5)));
    const close = vi.fn();
    const onOk = modalOk(run);
    onOk(close);
    onOk(close);
    await new Promise((r) => setTimeout(r, 20));
    expect(run).toHaveBeenCalledTimes(1);
    expect(close).toHaveBeenCalledTimes(1);
  });

  it('失败后可再次触发（pending 状态复位）', async () => {
    let fail = true;
    const run = vi.fn(async () => {
      if (fail) throw new Error('后端拒绝');
    });
    const close = vi.fn();
    const onOk = modalOk(run);
    onOk(close);
    await flush();
    expect(close).not.toHaveBeenCalled();
    fail = false;
    onOk(close);
    await flush();
    expect(run).toHaveBeenCalledTimes(2);
    expect(close).toHaveBeenCalledTimes(1);
  });

  it('支持自定义 errorText，空错误信息回退默认文案', async () => {
    modalOk(
      async () => {
        throw new Error('x');
      },
      () => '自定义失败提示',
    )(vi.fn());
    await flush();
    expect(message.error).toHaveBeenCalledWith('自定义失败提示');

    vi.clearAllMocks();
    modalOk(async () => {
      throw new Error('');
    })(vi.fn());
    await flush();
    expect(message.error).toHaveBeenCalledWith('操作失败');
  });
});
