package bannedwords

// Preset is one built-in banned word seeded for every tenant.
type Preset struct {
	Word       string
	Category   string
	Level      string
	Suggestion string
}

// CategoryLabel returns the Chinese label for a category code.
func CategoryLabel(category string) string {
	switch category {
	case CategoryAdExtreme:
		return "广告法极限词"
	case CategoryGeneral:
		return "通用违禁词"
	case CategoryMedical:
		return "医疗功效词"
	case CategoryInfringement:
		return "品牌侵权词"
	default:
		return "自定义"
	}
}

// LevelLabel returns the Chinese label for a severity level.
func LevelLabel(level string) string {
	switch level {
	case LevelForbidden:
		return "禁止"
	case LevelWarning:
		return "警告"
	default:
		return level
	}
}

// Categories lists all built-in category codes in display order.
func Categories() []string {
	return []string{CategoryAdExtreme, CategoryGeneral, CategoryMedical, CategoryInfringement}
}

// Presets lists built-in banned words seeded per tenant (readonly base library).
func Presets() []Preset {
	return []Preset{
		// 广告法极限词（《广告法》第九条：禁止使用绝对化用语）—— 禁止级
		{Word: "国家级", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "删除绝对化用语，或改为可证明的客观描述。"},
		{Word: "世界级", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "删除绝对化用语，或改为可证明的客观描述。"},
		{Word: "最高级", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "删除绝对化用语，或改为可证明的客观描述。"},
		{Word: "最佳", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "可改为「优选」「精选」等非绝对化表述。"},
		{Word: "最好", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "可改为「很好」「优质」等非绝对化表述。"},
		{Word: "最优", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "可改为「优选」「优质」等非绝对化表述。"},
		{Word: "最低价", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "可改为「优惠价」「特惠价」等表述。"},
		{Word: "全网最低", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "可改为「限时优惠」等可证明的促销表述。"},
		{Word: "史上最低", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "可改为「限时优惠」等可证明的促销表述。"},
		{Word: "第一", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "删除排名用语，或提供权威依据后改为具体数据。"},
		{Word: "第一品牌", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "删除排名用语，避免违反广告法。"},
		{Word: "销量第一", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "删除排名用语，或提供权威统计依据。"},
		{Word: "顶级", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "可改为「高端」「优质」等非绝对化表述。"},
		{Word: "极品", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "可改为「精品」「优品」等非绝对化表述。"},
		{Word: "绝无仅有", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "可改为「稀有」「少见」等非绝对化表述。"},
		{Word: "独一无二", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "可改为「独特」「别具一格」等表述。"},
		{Word: "空前绝后", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "删除绝对化用语。"},
		{Word: "万能", Category: CategoryAdExtreme, Level: LevelForbidden, Suggestion: "改为具体功能描述，避免夸大。"},
		{Word: "祖传", Category: CategoryAdExtreme, Level: LevelWarning, Suggestion: "建议改为可证明的工艺或历史描述。"},
		{Word: "特效", Category: CategoryAdExtreme, Level: LevelWarning, Suggestion: "建议改为具体效果描述并附依据。"},
		{Word: "百分百", Category: CategoryAdExtreme, Level: LevelWarning, Suggestion: "建议改为实际比例或删除绝对化表述。"},
		{Word: "100%", Category: CategoryAdExtreme, Level: LevelWarning, Suggestion: "涉及效果承诺时建议改为实际数据或删除。"},
		// 通用违禁词 —— 禁止级
		{Word: "假一赔十", Category: CategoryGeneral, Level: LevelWarning, Suggestion: "请确认承诺可兑现，部分平台禁止该表述。"},
		{Word: "点击领奖", Category: CategoryGeneral, Level: LevelForbidden, Suggestion: "删除诱导点击用语。"},
		{Word: "免费领取", Category: CategoryGeneral, Level: LevelWarning, Suggestion: "请确认活动真实存在，避免诱导性表述。"},
		{Word: "秒杀全网", Category: CategoryGeneral, Level: LevelForbidden, Suggestion: "删除贬低同行的比较性表述。"},
		{Word: "刷单", Category: CategoryGeneral, Level: LevelForbidden, Suggestion: "删除违规交易相关用语。"},
		{Word: "微信", Category: CategoryGeneral, Level: LevelWarning, Suggestion: "多数平台禁止引流到站外，请删除站外联系方式。"},
		{Word: "加V", Category: CategoryGeneral, Level: LevelForbidden, Suggestion: "删除站外引流用语。"},
		// 医疗功效词 —— 普通商品宣传医疗功效违规
		{Word: "治疗", Category: CategoryMedical, Level: LevelForbidden, Suggestion: "普通商品不得宣传医疗功效，请删除。"},
		{Word: "治愈", Category: CategoryMedical, Level: LevelForbidden, Suggestion: "普通商品不得宣传医疗功效，请删除。"},
		{Word: "根治", Category: CategoryMedical, Level: LevelForbidden, Suggestion: "普通商品不得宣传医疗功效，请删除。"},
		{Word: "药用", Category: CategoryMedical, Level: LevelForbidden, Suggestion: "普通商品不得宣传医疗功效，请删除。"},
		{Word: "抗菌", Category: CategoryMedical, Level: LevelWarning, Suggestion: "需有检测报告支撑，否则建议删除。"},
		{Word: "消炎", Category: CategoryMedical, Level: LevelForbidden, Suggestion: "普通商品不得宣传医疗功效，请删除。"},
		{Word: "降血压", Category: CategoryMedical, Level: LevelForbidden, Suggestion: "普通商品不得宣传医疗功效，请删除。"},
		{Word: "降血糖", Category: CategoryMedical, Level: LevelForbidden, Suggestion: "普通商品不得宣传医疗功效，请删除。"},
		{Word: "减肥", Category: CategoryMedical, Level: LevelWarning, Suggestion: "非特殊食品/器械不得宣传减肥功效，建议改为「塑形」等。"},
		{Word: "祛疤", Category: CategoryMedical, Level: LevelForbidden, Suggestion: "普通商品不得宣传医疗功效，请删除。"},
		// 品牌侵权词 —— 警告级（需人工确认是否授权）
		{Word: "正品代购", Category: CategoryInfringement, Level: LevelWarning, Suggestion: "请确认货源授权，避免侵权风险。"},
		{Word: "原单", Category: CategoryInfringement, Level: LevelForbidden, Suggestion: "删除涉假用语，平台禁止销售仿冒商品。"},
		{Word: "高仿", Category: CategoryInfringement, Level: LevelForbidden, Suggestion: "删除涉假用语，平台禁止销售仿冒商品。"},
		{Word: "A货", Category: CategoryInfringement, Level: LevelForbidden, Suggestion: "删除涉假用语，平台禁止销售仿冒商品。"},
		{Word: "尾单", Category: CategoryInfringement, Level: LevelWarning, Suggestion: "请确认货源合规，避免侵权风险。"},
	}
}
