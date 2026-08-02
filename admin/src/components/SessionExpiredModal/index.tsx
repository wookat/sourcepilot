import { LockOutlined, UserOutlined } from '@ant-design/icons';
import { Alert, Button, Form, Input, Modal, Space, message } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { formatUserErrorMessage } from '@/constants/errorMessages';
import { login, resolveSessionUser } from '@/services/auth';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import {
  redirectToLoginPage,
  registerReloginHandler,
  saveSessionCredentials,
} from '@/utils/sessionGuard';

type ApiErrorLike = {
  response?: { data?: { message?: string } };
  message?: string;
};

/**
 * 「登录已过期」重新登录弹窗：会话过期触发 401 时由 sessionGuard 唤起，
 * 在当前页内重新登录，不整页跳转，保留用户未提交的表单内容。
 */
export default function SessionExpiredModal() {
  const { initialState, setInitialState } = useInitialStateModel();
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form] = Form.useForm<{ account: string; password: string }>();
  const resolverRef = useRef<((ok: boolean) => void) | null>(null);

  const currentUser = initialState?.currentUser;

  useEffect(() => {
    registerReloginHandler(
      () =>
        new Promise<boolean>((resolve) => {
          resolverRef.current = resolve;
          setOpen(true);
        }),
    );
    return () => registerReloginHandler(null);
  }, []);

  useEffect(() => {
    if (!open) return;
    const account = currentUser?.email || currentUser?.username || '';
    if (account) form.setFieldsValue({ account });
  }, [open, currentUser, form]);

  const settle = (ok: boolean) => {
    setOpen(false);
    form.setFieldsValue({ password: '' });
    const resolve = resolverRef.current;
    resolverRef.current = null;
    resolve?.(ok);
  };

  const onRelogin = async () => {
    let values: { account: string; password: string };
    try {
      values = await form.validateFields();
    } catch {
      return;
    }
    setSubmitting(true);
    try {
      const data = await login(values.account.trim(), values.password);
      saveSessionCredentials(data);
      const nextUser = await resolveSessionUser(data);
      await setInitialState((s) => ({ ...s, currentUser: nextUser }));
      message.success('重新登录成功，可继续当前操作');
      settle(true);
    } catch (e: unknown) {
      const ax = e as ApiErrorLike;
      message.error(formatUserErrorMessage(ax?.response?.data?.message || ax?.message, '重新登录失败'));
    } finally {
      setSubmitting(false);
    }
  };

  const onGoLogin = () => {
    settle(false);
    redirectToLoginPage();
  };

  return (
    <Modal
      title="登录已过期"
      open={open}
      closable={false}
      maskClosable={false}
      keyboard={false}
      footer={
        <Space>
          <Button onClick={onGoLogin} disabled={submitting}>
            去登录页
          </Button>
          <Button type="primary" loading={submitting} onClick={onRelogin}>
            重新登录
          </Button>
        </Space>
      }
      destroyOnClose={false}
      forceRender
      width={400}
      /* 必须盖过业务弹窗（antd Modal 默认 z-index 1000）：会话过期常发生在业务 Modal 提交时 */
      zIndex={2000}
    >
      <Alert
        type="warning"
        showIcon
        message="登录状态已过期，重新登录后可继续当前操作，页面上未保存的内容不会丢失。"
        style={{ marginBottom: 16 }}
      />
      <Form form={form} layout="vertical" onFinish={onRelogin}>
        <Form.Item
          name="account"
          label="账号"
          rules={[{ required: true, message: '请输入邮箱或手机号' }]}
        >
          <Input prefix={<UserOutlined />} placeholder="邮箱 / 手机号" autoComplete="username" />
        </Form.Item>
        <Form.Item
          name="password"
          label="密码"
          rules={[{ required: true, message: '请输入密码' }]}
        >
          <Input.Password
            prefix={<LockOutlined />}
            placeholder="密码"
            autoComplete="current-password"
            onPressEnter={onRelogin}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}
