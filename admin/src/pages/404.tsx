import { Button, Result } from 'antd';
import { history } from '@umijs/max';

/** 404 副标题（与权限空态语义分离，见 components/RouteAccessGuard.tsx）。 */
export const NOT_FOUND_SUBTITLE = '你访问的地址不存在或已变更，请检查地址是否正确。';

export default function NotFoundPage() {
  return (
    <Result
      status="404"
      title="页面不存在"
      subTitle={NOT_FOUND_SUBTITLE}
      extra={
        <Button type="primary" onClick={() => history.push('/dashboard')}>
          返回工作台
        </Button>
      }
    />
  );
}
