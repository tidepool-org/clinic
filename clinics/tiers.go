package clinics

const DefaultTier = "tier0100"

var tiers = map[string]string{
	"tier0100": "Free",
	"tier0200": "Standard",
	"tier0300": "Premium",
	"tier0400": "Enterprise",
}

func GetTierDescription(tier string) string {
	if description, ok := tiers[tier]; ok {
		return description
	}

	return ""
}
