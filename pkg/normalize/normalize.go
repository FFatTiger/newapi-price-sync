package normalize

import "math"

func Round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

func EffectivePrice(price, exchangeRate, priceMultiplier float64) float64 {
	return price * exchangeRate * priceMultiplier
}

// NewAPI ratio semantics: 1 ratio == $2 / 1M input tokens.
func ModelRatioFromUSDPer1M(inputPrice, exchangeRate, priceMultiplier float64) float64 {
	return Round6(EffectivePrice(inputPrice, exchangeRate, priceMultiplier) / 2)
}

func ModelPriceFromUnitPrice(price, exchangeRate, priceMultiplier float64) float64 {
	return Round6(EffectivePrice(price, exchangeRate, priceMultiplier))
}

func Ratio(numerator, denominator float64) (float64, bool) {
	if denominator <= 0 {
		return 0, false
	}
	return Round6(numerator / denominator), true
}
