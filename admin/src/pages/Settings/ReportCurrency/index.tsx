import { GlobalOutlined, MinusCircleOutlined, PlusOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { ProCard } from '@ant-design/pro-components';
import { TmPageContainer } from '@/components/ui';
import { Alert, Button, Col, Form, Input, Row, Select, Space, Typography, message } from 'antd';
import { useCallback, useEffect, useState } from 'react';
import {
  fetchReportCurrencySettings,
  saveReportCurrencySettings,
  type ReportCurrencyRate,
} from '@/services/settings';

const { Paragraph } = Typography;

const CURRENCY_CODE_RE = /^[A-Za-z]{3}$/;
const RATE_RE = /^\d+(\.\d{1,6})?$/;

const BASE_CURRENCY_OPTIONS = ['CNY', 'USD', 'EUR', 'GBP', 'JPY', 'SGD', 'MYR', 'THB', 'VND', 'PHP', 'IDR'].map(
  (c) => ({ label: c, value: c }),
);

type FormValues = { baseCurrency: string; rates: ReportCurrencyRate[] };

export default function ReportCurrencySettingsPage() {
  const [form] = Form.useForm<FormValues>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const dto = await fetchReportCurrencySettings();
      form.setFieldsValue({ baseCurrency: dto.baseCurrency || 'CNY', rates: dto.rates ?? [] });
    } catch (e: unknown) {
      message.error((e as Error)?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [form]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <TmPageContainer
      title="报表本位币与汇率"
      subTitle="经营报表与首页经营概览按本位币统一折算展示；未配置汇率的币种将标记为「未折算」。"
    >
      <div className="tm-system-settings">
        <ProCard variant="outlined" className="tm-system-settings__hero">
          <div className="tm-system-settings__hero-inner">
            <div className="tm-system-settings__hero-icon">
              <GlobalOutlined />
            </div>
            <div className="tm-system-settings__hero-body">
              <Typography.Title level={5} className="tm-system-settings__hero-title">
                多币种订单 → 本位币报表
              </Typography.Title>
              <Paragraph type="secondary" className="tm-system-settings__hero-desc">
                手工维护「1 单位原币 = 多少本位币」的汇率表（不接实时汇率 API）。报表合计、每日趋势与 CSV
                导出中的折算列均使用该汇率表；缺少汇率的币种不会静默错算，而是列入「未折算」提示。
              </Paragraph>
            </div>
          </div>
        </ProCard>

        <Form<FormValues>
          form={form}
          layout="vertical"
          onFinish={async (vals) => {
            setSaving(true);
            try {
              const dto = await saveReportCurrencySettings({
                baseCurrency: vals.baseCurrency,
                rates: (vals.rates ?? []).map((r) => ({
                  currency: r.currency.trim().toUpperCase(),
                  rate: r.rate.trim(),
                })),
              });
              form.setFieldsValue({ baseCurrency: dto.baseCurrency, rates: dto.rates ?? [] });
              message.success('已保存');
            } catch (e: unknown) {
              message.error((e as Error)?.message || '保存失败');
            } finally {
              setSaving(false);
            }
          }}
        >
          <ProCard
            variant="outlined"
            title="本位币"
            className="tm-system-settings__panel"
            extra={
              <Button type="link" icon={<ReloadOutlined />} onClick={() => void load()} disabled={loading}>
                重新加载
              </Button>
            }
          >
            <Row gutter={[24, 0]}>
              <Col xs={24} md={12} lg={8}>
                <Form.Item
                  name="baseCurrency"
                  label="报表本位币"
                  rules={[{ required: true, message: '请选择本位币' }]}
                  extra="默认 CNY；本位币自身汇率恒为 1，无需配置"
                >
                  <Select options={BASE_CURRENCY_OPTIONS} showSearch />
                </Form.Item>
              </Col>
            </Row>
          </ProCard>

          <ProCard variant="outlined" title="手工汇率表" className="tm-system-settings__panel">
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message="汇率含义：1 单位原币 = 多少本位币。例如本位币 CNY 时，USD 填 7.13 表示 1 USD = 7.13 CNY。"
            />
            <Form.List name="rates">
              {(fields, { add, remove }) => (
                <>
                  {fields.map(({ key, name, ...rest }) => (
                    <Space key={key} align="baseline" wrap style={{ display: 'flex', marginBottom: 8 }}>
                      <Form.Item
                        {...rest}
                        name={[name, 'currency']}
                        rules={[
                          { required: true, message: '请输入币种代码' },
                          { pattern: CURRENCY_CODE_RE, message: '3 位货币代码，如 USD' },
                        ]}
                      >
                        <Input placeholder="币种（如 USD）" style={{ width: 160 }} maxLength={3} />
                      </Form.Item>
                      <Form.Item
                        {...rest}
                        name={[name, 'rate']}
                        rules={[
                          { required: true, message: '请输入汇率' },
                          { pattern: RATE_RE, message: '正的十进制数，最多 6 位小数' },
                        ]}
                      >
                        <Input placeholder="汇率（如 7.13）" style={{ width: 200 }} />
                      </Form.Item>
                      <Button
                        type="text"
                        icon={<MinusCircleOutlined />}
                        aria-label="删除该汇率"
                        disabled={loading}
                        onClick={() => remove(name)}
                      />
                    </Space>
                  ))}
                  {/* 加载中禁用：加载完成时 setFieldsValue 会重置 rates，提前新增的行会被覆盖 */}
                  <Button
                    type="dashed"
                    icon={<PlusOutlined />}
                    disabled={loading}
                    onClick={() => add({ currency: '', rate: '' })}
                  >
                    添加币种汇率
                  </Button>
                </>
              )}
            </Form.List>
          </ProCard>

          <ProCard variant="outlined" className="tm-system-settings__footer">
            <Button type="primary" htmlType="submit" loading={saving} disabled={loading} icon={<SaveOutlined />}>
              保存配置
            </Button>
          </ProCard>
        </Form>
      </div>
    </TmPageContainer>
  );
}
