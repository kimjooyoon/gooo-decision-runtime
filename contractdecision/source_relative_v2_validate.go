package contractdecision

import (
	"errors"
	"math"
)

func validateSourceRelativeV2(inputs [][528]float32, conditions ConditionSource) error {
	if err := validateSourceRelativeConditions(inputs, conditions); err != nil {
		return err
	}
	for _, row := range inputs {
		for i := range 3 {
			block := row[384+i*48 : 384+(i+1)*48]
			present := 0
			for j, value := range block {
				if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
					return errors.New("nonfinite V2 expression cell")
				}
				if j < 10 {
					if value == 1.0/8 {
						present++
					} else if value != 0 {
						return errors.New("V2 operation flag differs")
					}
				} else if j < 16 && value != 0 {
					return errors.New("reserved V2 operation cell populated")
				}
			}
			if present != 1 {
				return errors.New("V2 expression requires exactly one operation")
			}
		}
	}
	return nil
}
