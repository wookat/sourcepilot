import { Spin } from 'antd';

/** 路由懒加载 Suspense 兜底：居中加载态，避免切换页面时白屏闪烁 */
export default function PageLoading() {
  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        minHeight: 320,
        width: '100%',
      }}
    >
      <Spin />
    </div>
  );
}
