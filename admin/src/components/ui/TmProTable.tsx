import { ReloadOutlined } from '@ant-design/icons';
import { ProTable } from '@ant-design/pro-components';
import type { ActionType, ProColumns, ProTableProps } from '@ant-design/pro-components';
import { Button, Tooltip } from 'antd';
import { useCallback, useMemo, useRef, useState } from 'react';
import { useWideScreen } from '@/hooks/useWideScreen';

export type TmProTableProps<T extends Record<string, unknown>, U extends Record<string, unknown> = Record<string, unknown>> =
  ProTableProps<T, U>;

type TmToolBarRender<T extends Record<string, unknown>, U extends Record<string, unknown>> = Exclude<
  ProTableProps<T, U>['toolBarRender'],
  false | undefined
>;

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
 * - 窄屏（<768px）去除列 fixed，避免固定操作列盖住数据列，整表横向滑动查看；
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
  const wideScreen = useWideScreen();

  const mergedColumns = useMemo(() => {
    if (!columns || wideScreen) return columns;
    return stripColumnsFixed(columns);
  }, [columns, wideScreen]);

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
    <ProTable<T, U>
      {...rest}
      key={wideScreen ? 'tm-wide' : 'tm-narrow'}
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
  );
}
