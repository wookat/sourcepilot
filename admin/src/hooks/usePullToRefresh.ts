import { useEffect, useRef, useState } from 'react';

/** 触发刷新所需的下拉距离（px） */
const PULL_THRESHOLD = 72;

/**
 * 轻量下拉刷新：页面滚动到顶部时下拉超过阈值触发 onRefresh。
 * 返回容器 ref 与当前状态（用于展示「下拉刷新 / 松开刷新 / 刷新中」提示）。
 */
export function usePullToRefresh(onRefresh: () => Promise<void> | void) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [pulling, setPulling] = useState(false);
  const [ready, setReady] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const startY = useRef<number | null>(null);
  const refreshingRef = useRef(false);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return undefined;

    const scrollTop = () =>
      document.scrollingElement?.scrollTop ?? window.scrollY ?? 0;

    const onTouchStart = (e: TouchEvent) => {
      if (refreshingRef.current || scrollTop() > 0) return;
      startY.current = e.touches[0]?.clientY ?? null;
    };
    const onTouchMove = (e: TouchEvent) => {
      if (startY.current == null || refreshingRef.current) return;
      const dy = (e.touches[0]?.clientY ?? 0) - startY.current;
      if (dy > 8 && scrollTop() <= 0) {
        setPulling(true);
        setReady(dy >= PULL_THRESHOLD);
      } else {
        setPulling(false);
        setReady(false);
      }
    };
    const onTouchEnd = () => {
      if (startY.current == null) return;
      startY.current = null;
      setPulling(false);
      if (ready && !refreshingRef.current) {
        refreshingRef.current = true;
        setRefreshing(true);
        Promise.resolve(onRefresh()).finally(() => {
          refreshingRef.current = false;
          setRefreshing(false);
        });
      }
      setReady(false);
    };

    el.addEventListener('touchstart', onTouchStart, { passive: true });
    el.addEventListener('touchmove', onTouchMove, { passive: true });
    el.addEventListener('touchend', onTouchEnd);
    return () => {
      el.removeEventListener('touchstart', onTouchStart);
      el.removeEventListener('touchmove', onTouchMove);
      el.removeEventListener('touchend', onTouchEnd);
    };
  }, [onRefresh, ready]);

  return { containerRef, pulling, ready, refreshing };
}
