import type { KeyboardEvent, ReactElement, ReactNode } from 'react';
import { Suspense, lazy } from 'react';
import type { MenuDataItem } from '@umijs/route-utils';
import { history, request as umiRequest } from '@umijs/max';
import type { RequestConfig, RunTimeLayoutConfig } from '@/typings/umi-runtime';
import AppMessageBridge from '@/components/AppMessageBridge';
import BrandLogo from '@/components/BrandLogo';
import RouteAccessGuard from '@/components/RouteAccessGuard';
import { AUTH_TOKEN_KEY } from '@/constants/auth';
import { themeTokens } from '@/constants/layoutTokens';
import { fetchProfileWithTokenDetailed } from '@/services/auth';
import { isAuthStateUnavailable, retryWhileAuthStateUnavailable } from '@/utils/authStateRetry';
import { normalizeHttpErrorMessage } from '@/utils/httpErrorCopy';
import { filterMenuByPermission } from '@/utils/menuAccess';
import {
  clearSessionCredentials,
  isAuthUrl,
  redirectToLoginPage,
  refreshAccessToken,
  requireRelogin,
  shouldRefreshSoon,
} from '@/utils/sessionGuard';
import type { InitialState } from '@/typings/umi-runtime';

/** 重登弹窗与布局账号区不在首屏渲染路径上，懒加载以把 Form/Input/Dropdown 等剥离出首包 */
const SessionExpiredModal = lazy(() => import('@/components/SessionExpiredModal'));
const SiderUserFooter = lazy(() =>
  import('@/components/LayoutUserActions').then((m) => ({ default: m.SiderUserFooter })),
);
const RightActions = lazy(() =>
  import('@/components/LayoutUserActions').then((m) => ({ default: m.RightActions })),
);

/** ProLayout 侧栏菜单头部 / 头像区回调的常用 props */
type SiderMenuLayoutProps = {
  collapsed?: boolean;
};

/**
 * Runs inside umi antd innerProvider `<App>` (under ConfigProvider).
 * Do not add another `<App>` in rootContainer — that wraps outside ConfigProvider and breaks static message.
 */
export function innerProvider(container: ReactElement) {
  return (
    <>
      <AppMessageBridge />
      <Suspense fallback={null}>
        <SessionExpiredModal />
      </Suspense>
      {container}
    </>
  );
}

export async function getInitialState(): Promise<{ currentUser?: API.CurrentUser }> {
  const token = localStorage.getItem(AUTH_TOKEN_KEY);
  if (!token) {
    return {};
  }
  const profile = await fetchProfileWithTokenDetailed(token);
  if (profile.authStateUnavailable) {
    // 后端 fail-closed 瞬断：凭证仍有效，不清凭证，恢复后刷新即可续用
    return {};
  }
  if (!profile.user) {
    clearSessionCredentials();
    return {};
  }
  return { currentUser: profile.user };
}

function onLoginPage() {
  const path = history.location.pathname;
  return path === '/user/login' || path.startsWith('/user/login');
}

/** 401 时先静默续期、失败再弹「登录已过期」重登弹窗，成功后原样重放请求，避免整页跳转丢表单 */
async function handleUnauthorizedAndRetry(error: {
  config?: Record<string, unknown> & { url?: string; method?: string; headers?: unknown };
}) {
  const cfg = error?.config || {};
  let ok = await refreshAccessToken();
  if (!ok) ok = await requireRelogin();
  if (!ok) {
    redirectToLoginPage();
    // 整页跳登录页已接管：悬挂该请求的 Promise，避免页面代码未 catch 时触发 Unhandled Rejection 遮罩
    return new Promise<never>(() => {});
  }
  return umiRequest(String(cfg.url || ''), {
    method: (cfg.method as string) || 'GET',
    data: cfg.data,
    params: cfg.params as Record<string, string | number | boolean | undefined> | undefined,
    headers: { ...((cfg.headers as Record<string, string>) || {}) },
    sessionGuardRetry: true,
    skipErrorHandler: true,
    getResponse: true,
  });
}

export const request: RequestConfig = {
  requestInterceptors: [
    async (url, options) => {
      if (!isAuthUrl(url) && shouldRefreshSoon()) {
        // token 剩余有效期 < 5 分钟：先静默续期再发请求（single-flight，失败则继续用旧 token）
        await refreshAccessToken();
      }
      const token = localStorage.getItem(AUTH_TOKEN_KEY);
      const headers: Record<string, string> = {
        ...((options.headers as Record<string, string>) || {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      };
      return { url, options: { ...options, headers } };
    },
  ],
  responseInterceptors: [
    [
      (response: unknown) => response,
      async (error: {
        response?: { status?: number };
        config?: Record<string, unknown> & { url?: string };
      }) => {
        const status = error?.response?.status;
        const cfg = error?.config || {};
        const reqUrl = String(cfg.url || '');
        if (status !== 401 || isAuthUrl(reqUrl) || cfg.sessionGuardRetry || onLoginPage()) {
          throw error;
        }
        // 数据库瞬断 fail-closed（AUTH_STATE_UNAVAILABLE）：会话未失效，不走重登守卫，
        // 提示后指数退避自动重试，恢复后无感续用
        if (isAuthStateUnavailable(error)) {
          return retryWhileAuthStateUnavailable(error);
        }
        return handleUnauthorizedAndRetry(error);
      },
    ],
    [
      // 全站错误文案兜底：error.message 缺失或为 axios 英文原文时改写为后端结构化中文
      // message 或状态码中文兜底；不改动 response/status/config，专有处理（会话守卫、
      // 只读 403、AI 路径）继续按原字段分支，兜底只兜漏网。
      (response: unknown) => response,
      (error: unknown) => {
        throw normalizeHttpErrorMessage(error);
      },
    ],
  ],
  errorConfig: {
    errorHandler: (error: any) => {
      if (error?.info?.skipErrorHandler) {
        throw error;
      }
      const status = error?.response?.status;
      const reqUrl = String(error?.config?.url || '');
      // AUTH_STATE_UNAVAILABLE 不属于会话失效，重试链路自行处理，绝不跳登录页
      if (isAuthStateUnavailable(error)) {
        throw error;
      }
      // 重放后仍 401（如 token_version 已失效）才兜底跳登录页；首个 401 由 responseInterceptors 处理
      if (status === 401 && error?.config?.sessionGuardRetry && !reqUrl.includes('/auth/login')) {
        redirectToLoginPage();
        return new Promise<never>(() => {});
      }
      throw error;
    },
  },
};

/** 侧栏 / 顶栏品牌图形（与登录页同一 `logo.png`） */
const TM_BRAND_MARK = <BrandLogo height={28} />;

export const layout: RunTimeLayoutConfig = ({ initialState }) => ({
  title: false,
  logo: TM_BRAND_MARK,
  /** ProLayout 在侧栏会把 avatar 区域与（未定义 actionsRender 时的）rightContentRender 各渲染一遍，导致两行相同账号 */
  actionsRender: () => [],
  menuHeaderRender: (logoDom: ReactNode, _titleDom: ReactNode, props?: SiderMenuLayoutProps) => {
    const collapsed = props?.collapsed;
    const goHome = () => history.push('/dashboard');
    const interactive = {
      role: 'button' as const,
      tabIndex: 0,
      onClick: goHome,
      onKeyDown: (e: KeyboardEvent) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          goHome();
        }
      },
    };

    if (collapsed) {
      return (
        <div
          {...interactive}
          style={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            padding: '14px 0 10px',
            cursor: 'pointer',
            width: '100%',
          }}
        >
          {logoDom}
        </div>
      );
    }

    return (
      <div
        {...interactive}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '14px 16px 10px',
          cursor: 'pointer',
          width: '100%',
          minWidth: 0,
        }}
      >
        {logoDom}
        <span
          style={{
            fontWeight: 600,
            fontSize: 16,
            letterSpacing: '-0.02em',
            color: themeTokens.colorText,
            whiteSpace: 'nowrap',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
        >
          贸灵 <span style={{ fontWeight: 500, color: themeTokens.colorTextSecondary }}>TradeMind</span>
        </span>
      </div>
    );
  },
  avatarProps: initialState?.currentUser
    ? {
        render: (
          _avatarProps: Record<string, unknown>,
          _defaultDom: ReactNode,
          menuProps?: SiderMenuLayoutProps,
        ) => (
          <Suspense fallback={null}>
            <SiderUserFooter collapsed={menuProps?.collapsed} />
          </Suspense>
        ),
      }
    : false,
  token: {
    headerHeight: 56,
    colorBgLayout: themeTokens.colorBgLayout,
    colorTextMenuSelected: themeTokens.colorPrimary,
    colorBgMenuItemSelected: 'rgba(37, 99, 235, 0.09)',
    siderWidth: 224,
  },
  menu: { locale: false },
  childrenRender: (children: ReactNode) => <RouteAccessGuard>{children}</RouteAccessGuard>,
  menuDataRender: (menuData: MenuDataItem[]) =>
    filterMenuByPermission(
      menuData,
      initialState?.currentUser?.role,
      initialState?.currentUser?.permissions,
      initialState?.currentUser?.tenantId,
    ),
  onPageChange: () => {
    const { pathname } = history.location;
    if (pathname === '/user/login' || pathname.startsWith('/user/login')) return;
    // 必须用 token 判断：initialState 在此闭包里不会在登录后刷新，会一直当作未登录并反复 push 登录页，触发 Navigate 死循环。
    if (!localStorage.getItem(AUTH_TOKEN_KEY)) {
      history.replace(`/user/login?redirect=${encodeURIComponent(pathname)}`);
    }
  },
  rightContentRender: () => (
    <Suspense fallback={null} key="nav-right">
      <RightActions />
    </Suspense>
  ),
});
