import { listCarriers, type CarrierRow } from '@/services/carriers';
import { AutoComplete } from 'antd';
import { useEffect, useState } from 'react';

/**
 * Loads the tenant's enabled carriers once per mount. Failure degrades to an
 * empty list (free-text input still works).
 */
export function useEnabledCarriers(active = true): { carriers: CarrierRow[]; loading: boolean } {
  const [carriers, setCarriers] = useState<CarrierRow[]>([]);
  const [loading, setLoading] = useState(false);
  useEffect(() => {
    if (!active) return;
    let cancelled = false;
    setLoading(true);
    listCarriers({ enabled: true })
      .then((rows) => {
        if (!cancelled) setCarriers(rows);
      })
      .catch(() => {
        if (!cancelled) setCarriers([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [active]);
  return { carriers, loading };
}

/** Maps a carrier display name (or code) back to its row, if any. */
export function matchCarrier(carriers: CarrierRow[], value?: string): CarrierRow | undefined {
  const v = (value || '').trim();
  if (!v) return undefined;
  return carriers.find((c) => c.name === v || c.code === v.toLowerCase());
}

/**
 * Carrier picker with free-text fallback: selecting a preset fills its display
 * name; unknown names are kept as legacy free-text carriers.
 */
export default function CarrierSelect({
  value,
  onChange,
  carriers,
  loading,
  placeholder,
  disabled,
}: {
  value?: string;
  onChange?: (value: string) => void;
  carriers: CarrierRow[];
  loading?: boolean;
  placeholder?: string;
  disabled?: boolean;
}) {
  return (
    <AutoComplete
      value={value}
      onChange={(v) => onChange?.(v)}
      disabled={disabled}
      allowClear
      options={carriers.map((c) => ({ value: c.name, label: c.name }))}
      filterOption={(input, option) =>
        String(option?.value ?? '').toLowerCase().includes(input.trim().toLowerCase())
      }
      placeholder={placeholder ?? (loading ? '加载物流商…' : '选择物流商或输入承运商名称')}
    />
  );
}
