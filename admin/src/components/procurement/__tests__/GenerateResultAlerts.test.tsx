import { history } from '@umijs/max';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import GenerateResultAlerts from '../GenerateResultAlerts';

// 后端 generate 接口返回的 blocker 文案固定件（非 admin 侧 UI 直出文案）
const MSG_SKU_UNMATCHED = '订单行未匹配本地 SKU，请先完成 SKU 匹配';
const MSG_MAPPING_MISSING = '主货源缺少该 SKU 的映射，请先补全 SKU 映射';
const MSG_PRICE_MISSING = 'SKU 缺少参考进价';

describe('GenerateResultAlerts', () => {
  it('links sku.unmatched blockers to the order sku-match tab and closes the modal', async () => {
    const onNavigate = vi.fn();
    render(
      <GenerateResultAlerts
        blockers={[
          {
            orderId: 'order-1',
            code: 'sku.unmatched',
            message: MSG_SKU_UNMATCHED,
            skuName: 'red / L',
          },
        ]}
        onNavigate={onNavigate}
      />,
    );

    await userEvent.click(screen.getByRole('button', { name: '去规格匹配' }));
    expect(onNavigate).toHaveBeenCalledTimes(1);
    expect(history.push).toHaveBeenCalledWith('/orders/order-1?tab=sku');
  });

  it('links source.missing blockers to the product source profile with bind action', async () => {
    render(
      <GenerateResultAlerts
        blockers={[
          {
            orderId: 'order-1',
            productId: 'prod-1',
            localSkuId: 'sku-1',
            code: 'source.missing',
            message: '商品没有主货源，请先在货源档案绑定',
          },
        ]}
      />,
    );

    await userEvent.click(screen.getByRole('button', { name: '去绑定主货源' }));
    expect(history.push).toHaveBeenCalledWith('/sourcing/product-sources?productId=prod-1&action=bind');
  });

  it('links mapping.missing blockers to the mapping drawer with skuId locate param', async () => {
    render(
      <GenerateResultAlerts
        blockers={[
          {
            orderId: 'order-1',
            productId: 'prod-1',
            localSkuId: 'sku-1',
            code: 'mapping.missing',
            message: MSG_MAPPING_MISSING,
          },
        ]}
      />,
    );

    await userEvent.click(screen.getByRole('button', { name: '去补 SKU 映射' }));
    expect(history.push).toHaveBeenCalledWith(
      '/sourcing/product-sources?productId=prod-1&action=mapping&skuId=sku-1',
    );
  });

  it('renders plain text without an action link when structured ids are missing', () => {
    render(
      <GenerateResultAlerts
        blockers={[
          {
            orderId: 'order-1',
            code: 'source.missing',
            message: '商品没有主货源，请先在货源档案绑定',
          },
        ]}
        warnings={[{ orderId: 'order-1', code: 'price.missing', message: MSG_PRICE_MISSING }]}
      />,
    );

    expect(screen.queryByRole('button')).toBeNull();
    expect(screen.getByText(/商品没有主货源/)).toBeInTheDocument();
    expect(screen.getByText(/缺少参考进价/)).toBeInTheDocument();
  });
});
