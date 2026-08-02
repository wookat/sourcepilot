import { ReloadOutlined } from '@ant-design/icons';
import { ProTable } from '@ant-design/pro-components';
import type { ActionType, ProColumns, ProTableProps } from '@ant-design/pro-components';
import { Button, Tooltip } from 'antd';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

export type TmProTableProps<T extends Record<string, unknown>, U extends Record<string, unknown> = Record<string, unknown>> =
  ProTableProps<T, U>;

type TmToolBarRender<T extends Record<string, unknown>, U extends Record<string, unknown>> = Exclude<
  ProTableProps<T, U>['toolBarRender'],
  false | undefined
>;

/**
 * 固定列可用的最小内容区宽度（px）。口径用表格容器实际宽度而非视口宽度：
 * 同一视口下侧边栏展开/收起对可用宽度影响很大（768 视口恰逢侧边栏塌缩、
 * 834 视口带侧边栏时数据区仅剩三列），按容器宽判断不依赖侧边栏状态推断，更稳。
 */
const FIXED_COLUMNS_MIN_WIDTH = 768;

/** 监听表格容器宽度，容器宽度 > 768px 才启用列 fixed（惰性初始化保证首帧口径接近真实宽度） */
function useFixedColumnsEnabled(): [React.RefObject<HTMLDivElement>, boolean] {
  const ref = useRef<HTMLDivElement>(null);
  const [enabled, setEnabled] = useState(
    () => typeof window === 'undefined' || window.innerWidth > FIXED_COLUMNS_MIN_WIDTH,
  );
  useEffect(() => {
    const el = ref.current;
    if (!el) return undefined;
    const update = () => setEnabled(el.clientWidth > FIXED_COLUMNS_MIN_WIDTH);
    update();
    if (typeof ResizeObserver === 'undefined') return undefined;
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  return [ref, enabled];
}

function stripColumnsFixed<T, U>(columns: ProColumns<T, U>[]): ProColumns<T, U>[] {
  return columns.map((col) => {
    const next: ProColumns<T, U> = { ...col, fixed: undefined };
    const children = (col as { children?: ProColumns<T, U>[] }).children;
    if (Array.isArray(children)) {
      (next as { children?: ProColumns<T, U>[] }).children = stripColumnsFixed(children);
    }
    return next;
  });
}

/**
 * 统一 ProTable：
 * - 用可点击的 Button 承接刷新（修复工具栏内置 span 图标在某些布局下点击无效的问题）；
 * - 内容区窄（表格容器 ≤768px）去除列 fixed，避免固定操作列盖住数据列，整表横向滑动查看；
 * - 筛选区默认收起，列表页口径全站一致（页面可显式覆盖）。
 */
export default function TmProTable<
  T extends Record<string, unknown>,
  U extends Record<string, unknown> = Record<string, unknown>,
>({
  actionRef: userActionRef,
  options,
  toolBarRender,
  onLoadingChange,
  className,
  columns,
  search,
  ...rest
}: TmProTableProps<T, U>) {
  const innerRef = useRef<ActionType>();
  const actionRef = userActionRef ?? innerRef;
  const [loading, setLoading] = useState(false);
  const [containerRef, fixedEnabled] = useFixedColumnsEnabled();

  const mergedColumns = useMemo(() => {
    if (!columns || fixedEnabled) return columns;
    return stripColumnsFixed(columns);
  }, [columns, fixedEnabled]);

  const mergedSearch = useMemo(() => {
    if (search === false || search === undefined) return search;
    return { defaultCollapsed: true, ...search };
  }, [search]);

  const mergedOptions = useMemo(() => {
    if (options === false) {
      return false;
    }
    const base = options ?? {};
    return {
      density: true,
      setting: true,
      ...base,
      // 内置 reload 为 span+图标，点击区域易失效；改由 toolBarRender 中的 Button 触发。
      reload: false,
    };
  }, [options]);

  const mergedToolBarRender = useCallback(
    (action: ActionType | undefined, config: Parameters<TmToolBarRender<T, U>>[1]) => {
      const userNodes = typeof toolBarRender === 'function' ? toolBarRender(action, config) ?? [] : [];
      if (mergedOptions === false) {
        return userNodes;
      }
      return [
        ...userNodes,
        <Tooltip key="tm-reload" title="刷新">
          <Button
            type="text"
            aria-label="刷新"
            icon={<ReloadOutlined spin={loading} />}
            onClick={() => {
              void action?.reload?.();
            }}
          />
        </Tooltip>,
      ];
    },
    [toolBarRender, loading, mergedOptions],
  );

  return (
    <div ref={containerRef} className="tm-pro-table-container">
      <ProTable<T, U>
        {...rest}
        key={fixedEnabled ? 'tm-wide' : 'tm-narrow'}
        columns={mergedColumns}
        search={mergedSearch}
        className={['tm-pro-table', className].filter(Boolean).join(' ')}
        actionRef={actionRef}
        options={mergedOptions}
        toolBarRender={mergedToolBarRender}
        onLoadingChange={(isLoading) => {
          setLoading(!!isLoading);
          onLoadingChange?.(isLoading);
        }}
      />
    </div>
  );
}
