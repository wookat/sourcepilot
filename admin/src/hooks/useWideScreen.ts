import { Grid } from 'antd';
import { useEffect, useState } from 'react';

/** 宽屏判断（≥768px）：惰性初始化保证移动端首帧即窄屏（pro-table columnsMap 会固化首帧列 fixed）。 */
export function useWideScreen(): boolean {
  const screens = Grid.useBreakpoint();
  const [wide, setWide] = useState(() => typeof window === 'undefined' || window.innerWidth >= 768);
  useEffect(() => {
    if (screens.md !== undefined) setWide(screens.md);
  }, [screens.md]);
  return wide;
}
