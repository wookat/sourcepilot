import { Button, Result } from 'antd';
import { history } from '@umijs/max';
import { ROUTE_FALLBACK_SUBTITLE } from '@/components/RouteAccessGuard';

export default function NotFoundPage() {
  return (
    <Result
      status="404"
      title="无法访问该页面"
      subTitle={ROUTE_FALLBACK_SUBTITLE}
      extra={
        <Button type="primary" onClick={() => history.push('/dashboard')}>
          返回工作台
        </Button>
      }
    />
  );
}
