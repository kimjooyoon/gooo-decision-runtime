package decision

import "errors"

const OrderedExpressionV2FeatureVersion = "ordered_predicate_return_expressions_v2"

// V2 keeps all V1 cells and adds divide at operation cell 9. V1 continues to
// reject divide; callers must explicitly select this ABI for new model inputs.
func OrderedExpressionV2FeaturesInto(expressions [3]OrderedExpression,
	out *[OrderedExpressionFeatureDim]float32) error {
	if out == nil {
		return errors.New("ordered expression destination required")
	}
	normalized := expressions
	divisions := [3]bool{}
	for i, e := range expressions {
		if e.Operation == "divide" {
			for _, atom := range e.Operands {
				if !atom.Present || atom.Kind != "int" && atom.Kind != "input" || validateFlowValue(atom) != nil {
					return errors.New("ordered division requires two known Integer atoms")
				}
			}
			divisions[i] = true
			normalized[i].Operation = "multiply"
		}
	}
	var next [OrderedExpressionFeatureDim]float32
	if err := OrderedExpressionFeaturesInto(normalized, &next); err != nil {
		return err
	}
	for i, division := range divisions {
		if division {
			next[i*48+3] = 0
			next[i*48+9] = 1.0 / 8
		}
	}
	*out = next
	return nil
}
