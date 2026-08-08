package settings

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Controlled-value keys are settings items whose stored value gates a
// security-relevant behaviour and therefore gets a server-side value-domain
// check at write time (the UI form constraints alone are bypassable). An
// empty value stays legal: it means "unset" and every consumer of these keys
// fails closed on unset values.

// markPaidLimitMax caps the mark-paid ceilings at 1e10 (100 亿), the same
// hard bound amountCents enforces on individual mark-paid amounts; a limit
// above it could never be reached and only hides misconfiguration.
const markPaidLimitMax = 1e10

// markPaidLimitPattern accepts plain decimal money values only: digits with
// at most two decimals. Scientific notation, signs, hex, separators and any
// trailing garbage are rejected.
var markPaidLimitPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]{1,2})?$`)

func isMarkPaidLimitKey(groupKey, itemKey string) bool {
	if !strings.EqualFold(groupKey, "mcp") {
		return false
	}
	return strings.EqualFold(itemKey, "mark_paid_single_limit") ||
		strings.EqualFold(itemKey, "mark_paid_daily_limit")
}

// validateControlledValue rejects out-of-domain values for controlled keys.
// It returns the canonical (trimmed) value to store.
func validateControlledValue(it PutItem, groupKey, itemKey string) (string, error) {
	if !isMarkPaidLimitKey(groupKey, itemKey) {
		return it.ItemValue, nil
	}
	if it.IsEncrypted {
		return "", fmt.Errorf("mcp/%s 不支持加密存储：该金额上限为受控数值项，请以明文数字保存", itemKey)
	}
	if it.Clear {
		return "", nil
	}
	val := strings.TrimSpace(it.ItemValue)
	if val == "" {
		return "", nil // unset: consumers fail closed
	}
	if !markPaidLimitPattern.MatchString(val) {
		return "", fmt.Errorf("mcp/%s 金额上限格式非法：必须为大于 0、至多两位小数的十进制数字（不接受科学计数法、正负号、千分位或非数字字符）", itemKey)
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil || f <= 0 {
		return "", fmt.Errorf("mcp/%s 金额上限必须大于 0", itemKey)
	}
	if f > markPaidLimitMax {
		return "", fmt.Errorf("mcp/%s 金额上限不得超过 10000000000（100 亿），超过该值的上限等同于不设限", itemKey)
	}
	return val, nil
}
