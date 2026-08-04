package order

import (
	"errors"
	"testing"
)

func TestIsOrderNoUniqueViolation(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{
			"postgres duplicate key on tenant order_no index",
			errors.New(`ERROR: duplicate key value violates unique constraint "idx_orders_tenant_order_no" (SQLSTATE 23505)`),
			true,
		},
		{
			"other unique index",
			errors.New(`ERROR: duplicate key value violates unique constraint "idx_orders_shop_platform_external" (SQLSTATE 23505)`),
			false,
		},
		{"unrelated error", errors.New("connection refused"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOrderNoUniqueViolation(tc.err); got != tc.want {
				t.Fatalf("isOrderNoUniqueViolation(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
