import { queryWarehouses, type WarehouseDTO } from '@/services/inventory';
import { Select, type SelectProps } from 'antd';
import { useEffect, useState } from 'react';

export type WarehouseSelectProps = Omit<SelectProps<string>, 'options' | 'loading'> & {
  /** 是否包含「全部仓库」选项（value 为空字符串） */
  includeAll?: boolean;
  /** 是否包含已停用仓库（默认不含；含时选项禁用） */
  includeDisabled?: boolean;
  /** 加载完成后自动选中默认仓（仅当当前无值时） */
  preselectDefault?: boolean;
  onWarehousesLoaded?: (list: WarehouseDTO[]) => void;
};

/** 仓库下拉选择：租户仓库列表，默认仓标注（默认） */
export default function WarehouseSelect({
  includeAll,
  includeDisabled,
  preselectDefault,
  onWarehousesLoaded,
  value,
  onChange,
  ...rest
}: WarehouseSelectProps) {
  const [warehouses, setWarehouses] = useState<WarehouseDTO[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let mounted = true;
    setLoading(true);
    queryWarehouses()
      .then((res) => {
        if (!mounted) return;
        const list = res.list ?? [];
        setWarehouses(list);
        onWarehousesLoaded?.(list);
        if (preselectDefault && !value) {
          const def = list.find((w) => w.isDefault);
          if (def) onChange?.(def.id, { label: def.name, value: def.id });
        }
      })
      .catch(() => undefined)
      .finally(() => {
        if (mounted) setLoading(false);
      });
    return () => {
      mounted = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const options = [
    ...(includeAll ? [{ label: '全部仓库', value: '' }] : []),
    ...warehouses
      .filter((w) => includeDisabled || w.enabled)
      .map((w) => ({
        label: w.isDefault ? `${w.name}（默认）` : w.enabled ? w.name : `${w.name}（已停用）`,
        value: w.id,
        disabled: !w.enabled,
      })),
  ];

  return (
    <Select<string>
      placeholder="选择仓库"
      allowClear={!preselectDefault}
      {...rest}
      value={value}
      onChange={onChange}
      loading={loading}
      options={options}
    />
  );
}
