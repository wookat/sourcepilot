import type { CSSProperties } from 'react';
import { LogoutOutlined } from '@ant-design/icons';
import { Avatar, Dropdown, Space, Tooltip } from 'antd';
import { history } from '@umijs/max';
import { themeTokens, tmSemanticTokens } from '@/constants/layoutTokens';
import { postJSON } from '@/services/request';
import { clearSessionCredentials } from '@/utils/sessionGuard';
import { useInitialStateModel } from '@/hooks/useInitialStateModel';
import type { InitialStateModel } from '@/typings/umi-runtime';

const TM_AVATAR_GRADIENT_BG = `linear-gradient(135deg, ${themeTokens.colorPrimary} 0%, ${tmSemanticTokens.dataAccent} 100%)`;

const TM_AVATAR_STYLE: CSSProperties = { background: TM_AVATAR_GRADIENT_BG };

async function logoutAndClear(
  setInitialState: InitialStateModel['setInitialState'],
) {
  try {
    await postJSON('/api/v1/auth/logout');
  } catch {
    /* ignore */
  }
  clearSessionCredentials();
  setInitialState((s) => ({ ...s, currentUser: undefined }));
  history.push('/user/login');
}

function looksLikeEmail(value: string) {
  return value.includes('@');
}

/** 侧栏/顶栏展示：邮箱账号优先显示 @ 前昵称，完整邮箱放副行 */
function resolveUserLabels(user?: API.CurrentUser) {
  const displayName = user?.displayName?.trim() || '管理员';
  const email = user?.email?.trim() || '';
  const username = user?.username?.trim() || '';
  const loginId = email || username;

  if (looksLikeEmail(displayName) && loginId && displayName === loginId) {
    const local = displayName.split('@')[0]?.trim() || displayName;
    return {
      primary: local,
      secondary: displayName,
      initial: local.slice(0, 1).toUpperCase(),
    };
  }

  return {
    primary: displayName,
    secondary: loginId && loginId !== displayName ? loginId : '',
    initial: displayName.slice(0, 1).toUpperCase(),
  };
}

function buildLogoutMenu(setInitialState: InitialStateModel['setInitialState']) {
  return {
    items: [
      {
        key: 'logout',
        icon: <LogoutOutlined />,
        label: '退出登录',
        onClick: () => void logoutAndClear(setInitialState),
      },
    ],
  };
}

/** 侧栏底部账号：整行可点，向上弹出菜单；邮箱账号双行展示避免截断 */
export function SiderUserFooter({ collapsed }: { collapsed?: boolean }) {
  const { setInitialState, initialState } = useInitialStateModel();
  const { primary, secondary, initial } = resolveUserLabels(initialState?.currentUser);
  const menu = buildLogoutMenu(setInitialState);
  const tooltipTitle = secondary ? `${primary}\n${secondary}` : primary;

  const avatar = (
    <Avatar size={32} style={TM_AVATAR_STYLE}>
      {initial}
    </Avatar>
  );

  const body = (
    <div className="tm-sider-user">
      {avatar}
      <div className="tm-sider-user__meta">
        <span className="tm-sider-user__name" title={primary}>
          {primary}
        </span>
        {secondary ? (
          <span className="tm-sider-user__sub" title={secondary}>
            {secondary}
          </span>
        ) : (
          <span className="tm-sider-user__sub">管理员</span>
        )}
      </div>
    </div>
  );

  return (
    <Dropdown
      menu={menu}
      trigger={['click']}
      placement={collapsed ? 'topRight' : 'topLeft'}
      overlayStyle={{ minWidth: 140 }}
    >
      <div
        className="tm-sider-user-trigger"
        role="button"
        tabIndex={0}
        aria-label={`当前用户 ${primary}`}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            (e.currentTarget as HTMLDivElement).click();
          }
        }}
      >
        {collapsed ? (
          <Tooltip title={tooltipTitle} placement="right">
            <span className="tm-sider-user tm-sider-user--collapsed">{avatar}</span>
          </Tooltip>
        ) : (
          body
        )}
      </div>
    </Dropdown>
  );
}

export function RightActions() {
  const { setInitialState, initialState } = useInitialStateModel();
  const { primary, initial } = resolveUserLabels(initialState?.currentUser);

  return (
    <Dropdown menu={buildLogoutMenu(setInitialState)} placement="bottomRight" trigger={['click']}>
      <Space size={10} className="tm-header-user" style={{ cursor: 'pointer', paddingInline: 4 }}>
        <Avatar size={32} style={{ ...TM_AVATAR_STYLE, fontSize: 14 }}>
          {initial}
        </Avatar>
        <span className="tm-layout-user-name" title={primary}>
          {primary}
        </span>
      </Space>
    </Dropdown>
  );
}
