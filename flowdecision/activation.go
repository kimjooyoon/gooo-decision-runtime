package flowdecision

const (
	ReLUActivation              = "relu_v1"
	LeakyReLUActivation         = "leaky_relu_0.01_v1"
	negativeSlope       float32 = .01
)

// Activation identifies the computation bound to these immutable weights.
// Earlier artifacts and constructors retain ReLU and their original identity.
func (m *Model) Activation() string {
	if m == nil {
		return ""
	}
	if m.activation == "" {
		return ReLUActivation
	}
	return m.activation
}

// ArtifactSchema identifies the closed serialized computation contract.
func (m *Model) ArtifactSchema() string {
	if m == nil {
		return ""
	}
	if m.Activation() == LeakyReLUActivation {
		return ActivationSchema
	}
	return Schema
}

func supportedActivation(name string) bool {
	return name == ReLUActivation || name == LeakyReLUActivation
}

func (m *Model) activate(x float32) float32 {
	if m.Activation() == LeakyReLUActivation && x < 0 {
		return x * negativeSlope
	}
	return max(x, 0)
}
