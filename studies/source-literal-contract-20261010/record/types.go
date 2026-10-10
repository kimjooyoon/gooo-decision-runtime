package record

type Projection struct {
	ID       string
	Literals []int64
	Cases    [][32]float32
}
