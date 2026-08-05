import {
  AlertOutlined,
  AuditOutlined,
  BarChartOutlined,
  DesktopOutlined,
  ExceptionOutlined,
  LogoutOutlined,
  RightOutlined,
  SettingOutlined,
} from '@ant-design/icons';
import { history } from '@umijs/max';
import { Avatar, Button, Typography } from 'antd';
import { usePermission } from '@/hooks/usePermission';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import { canAccessPath } from '@/utils/menuAccess';
import { postJSON } from '@/services/request';
import { clearSessionCredentials } from '@/utils/sessionGuard';

const ROLE_LABELS: Record<string, string> = {
  admin: '管理员',
  operator: '运营',
  readonly: '只读',
  platform_admin: '平台管理员',
};

/** 移动端「我的」：账号信息 + 常用入口 + 退出登录 */
export default function MobileMe() {
  const { user, role, permissions } = usePermission();
  const { setInitialState } = useInitialStateModel();

  const displayName = user?.displayName?.trim() || user?.username?.trim() || '管理员';
  const loginId = user?.email?.trim() || user?.username?.trim() || '';

  const links = [
    { title: '经营报表', path: '/orders/reports', icon: <BarChartOutlined /> },
    { title: '异常工作台', path: '/orders/exceptions', icon: <ExceptionOutlined /> },
    { title: '告警中心', path: '/ops/task-center/alerts', icon: <AlertOutlined /> },
    { title: '操作日志', path: '/system/operation-logs', icon: <AuditOutlined /> },
    { title: '系统设置', path: '/settings/system', icon: <SettingOutlined /> },
    { title: '桌面版工作台', path: '/dashboard', icon: <DesktopOutlined /> },
  ].filter((item) => canAccessPath(item.path, role, permissions, user?.tenantId));

  const logout = async () => {
    try {
      await postJSON('/api/v1/auth/logout');
    } catch {
      /* 登出接口失败也照常清凭证 */
    }
    clearSessionCredentials();
    setInitialState((s) => ({ ...s, currentUser: undefined }));
    history.push('/user/login');
  };

  return (
    <div className="tm-mobile-home" data-testid="tm-mobile-me">
      <div className="tm-mobile-me__profile">
        <Avatar size={48} style={{ background: 'var(--ant-color-primary)' }}>
          {displayName.slice(0, 1).toUpperCase()}
        </Avatar>
        <div style={{ minWidth: 0 }}>
          <Typography.Text strong style={{ display: 'block', fontSize: 16 }}>
            {displayName}
          </Typography.Text>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {[ROLE_LABELS[role || ''] || role || '', loginId].filter(Boolean).join(' · ')}
          </Typography.Text>
        </div>
      </div>

      {links.length ? (
        <div className="tm-mobile-section">
          <div className="tm-mobile-list">
            {links.map((item) => (
              <button
                key={item.path}
                type="button"
                className="tm-mobile-list-row"
                onClick={() => history.push(item.path)}
              >
                <span className="tm-mobile-list-row__title">
                  <span style={{ marginInlineEnd: 8 }}>{item.icon}</span>
                  {item.title}
                </span>
                <RightOutlined className="tm-mobile-list-row__arrow" />
              </button>
            ))}
          </div>
        </div>
      ) : null}

      <Button
        danger
        block
        icon={<LogoutOutlined />}
        className="tm-mobile-touch-btn"
        style={{ marginTop: 20 }}
        onClick={() => void logout()}
      >
        退出登录
      </Button>
    </div>
  );
}
