package export

import (
	"fmt"
	"math"
	"time"
)

const (
	MmolLToMgdLConversionFactor float64 = 18.01559
	MmolLToMgdLPrecisionFactor  float64 = 100000.0
)

func pfloat(f *float64, precision int) string {
	if f == nil {
		return ""
	}
	shift := math.Pow(10, float64(precision))
	return fmt.Sprintf("%v", math.RoundToEven(*f*shift)/shift)
}

func pint(i *int) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%v", *i)
}

func ptomgdl(valMmolL *float64) string {
	if valMmolL == nil || *valMmolL < math.SmallestNonzeroFloat64 {
		return ""
	}
	val := toMgDl(*valMmolL)
	return pfloat(&val, 0)
}

func ppct(f *float64, precision int) string {
	if f == nil {
		return ""
	}
	shift := math.Pow(10, float64(precision))
	return fmt.Sprintf("%v", math.RoundToEven(*f*shift*100)/shift)
}

func ptime(t *time.Time, layout string) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format(layout)
}
func ptimed(t *time.Time, layout, defaultVal string) string {
	if t == nil || t.IsZero() {
		return defaultVal
	}
	return t.Format(layout)
}

func pstr(p *string) string {
	if p == nil {
		return ""
	}

	return *p
}
func pstrd(p *string, defaultVal string) string {
	if p == nil || *p == "" {
		return defaultVal
	}

	return *p
}

func strp(s string) *string {
	return &s
}

func toMgDl(valMmolL float64) float64 {
	intValue := int(valMmolL*MmolLToMgdLConversionFactor*MmolLToMgdLPrecisionFactor + 0.5)
	return float64(intValue) / MmolLToMgdLPrecisionFactor
}
