package carrier

import (
	"fmt"
	"regexp"
	"strings"
)

// Preset is one built-in domestic carrier seeded for every tenant.
type Preset struct {
	Code                string
	Name                string
	TrackingURLTemplate string
	SortOrder           int
}

// Presets lists common domestic couriers seeded per tenant.
func Presets() []Preset {
	return []Preset{
		{Code: "sf", Name: "顺丰速运", TrackingURLTemplate: "https://www.sf-express.com/chn/sc/waybill/waybill-detail/{trackingNo}", SortOrder: 10},
		{Code: "jd", Name: "京东物流", TrackingURLTemplate: "https://www.jdl.com/orderSearch/?waybillCodes={trackingNo}", SortOrder: 20},
		{Code: "zto", Name: "中通快递", TrackingURLTemplate: "https://www.zto.com/express/expressCheck.html?txtBill={trackingNo}", SortOrder: 30},
		{Code: "yto", Name: "圆通速递", TrackingURLTemplate: "https://www.yto.net.cn/gw/service/Consult.html?number={trackingNo}", SortOrder: 40},
		{Code: "sto", Name: "申通快递", TrackingURLTemplate: "https://www.sto.cn/pc/service-page?type=1&billCode={trackingNo}", SortOrder: 50},
		{Code: "yunda", Name: "韵达速递", TrackingURLTemplate: "https://www.yundaex.com/cn/track/index?number={trackingNo}", SortOrder: 60},
		{Code: "ems", Name: "邮政EMS", TrackingURLTemplate: "https://www.ems.com.cn/queryList?mailNum={trackingNo}", SortOrder: 70},
		{Code: "jt", Name: "极兔速递", TrackingURLTemplate: "https://www.jtexpress.cn/trajectoryQuery?bills={trackingNo}", SortOrder: 80},
		{Code: "deppon", Name: "德邦快递", TrackingURLTemplate: "https://www.deppon.com/track?trackingNo={trackingNo}", SortOrder: 90},
		{Code: "other", Name: "其他快递", SortOrder: 999},
	}
}

// trackingNoPatterns holds per-carrier loose waybill formats. Only carriers
// with a confidently-known format get a specific rule; everything else falls
// back to the generic rule so odd-but-real numbers are never rejected.
var trackingNoPatterns = map[string]*regexp.Regexp{
	"sf":  regexp.MustCompile(`^SF[0-9]{10,15}$`),
	"jd":  regexp.MustCompile(`^JD[A-Z0-9]{10,18}$`),
	"ems": regexp.MustCompile(`^[A-Z]{2}[0-9]{9}[A-Z]{2}$`),
}

// genericTrackingNo accepts any 6-40 char alphanumeric (with dashes) waybill.
var genericTrackingNo = regexp.MustCompile(`^[A-Za-z0-9-]{6,40}$`)

// ValidateTrackingNo loosely checks a waybill number against the carrier's
// known format. Empty tracking numbers are allowed (pending shipments).
func ValidateTrackingNo(carrierCode, trackingNo string) error {
	tn := strings.TrimSpace(trackingNo)
	if tn == "" {
		return nil
	}
	code := strings.ToLower(strings.TrimSpace(carrierCode))
	if re, ok := trackingNoPatterns[code]; ok {
		if !re.MatchString(strings.ToUpper(tn)) {
			return fmt.Errorf("运单号格式与所选物流商不符（%s）", tn)
		}
		return nil
	}
	if !genericTrackingNo.MatchString(tn) {
		return fmt.Errorf("运单号格式无效（仅允许 6-40 位字母、数字或中划线）")
	}
	return nil
}
