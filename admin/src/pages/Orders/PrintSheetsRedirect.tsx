import { history, useLocation } from '@umijs/max';
import React, { useEffect } from 'react';

/** /orders/print-sheets 别名：重定向到 /orders/print 并保留查询参数（如 ids）。 */
const PrintSheetsRedirect: React.FC = () => {
  const location = useLocation();
  useEffect(() => {
    history.replace({ pathname: '/orders/print', search: location.search });
  }, [location.search]);
  return null;
};

export default PrintSheetsRedirect;
