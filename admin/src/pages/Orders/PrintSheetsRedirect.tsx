import { Navigate, useLocation } from '@umijs/max';
import React from 'react';

/** /orders/print-sheets 别名：重定向到 /orders/print 并保留查询参数（如 ids）。 */
const PrintSheetsRedirect: React.FC = () => {
  const location = useLocation();
  return <Navigate to={{ pathname: '/orders/print', search: location.search }} replace />;
};

export default PrintSheetsRedirect;
