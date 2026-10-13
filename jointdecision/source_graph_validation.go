package jointdecision

import "errors"

// ValidateSourceGraphNodes checks the existing bounded source graph carrier.
// It validates shape and backward edges, without executing or typing the code.
// Model contracts must still specify their choices, input roles and features.
func ValidateSourceGraphNodes(nodes []RecordGraphNode) error {
	if len(nodes) == 0 || len(nodes) > RecordGraphNodeLimit {
		return errors.New("source graph requires 1..512 nodes")
	}
	for i, node := range nodes {
		if err := validateGraphNode(node, uint16(i+1)); err != nil {
			return err
		}
	}
	return nil
}
