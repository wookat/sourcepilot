// Package reports serves the deep report read APIs (profit / procurement /
// inventory). It is a read-only aggregation layer: it queries other modules'
// tables through their exported models, converts multi-currency amounts to
// the tenant report base currency via the manual fxrate table (settings group
// report_currency), and never writes business data. All endpoints apply
// tenant scope; order-based reports additionally apply the caller's store
// scope so operators only see authorized shops.
package reports
