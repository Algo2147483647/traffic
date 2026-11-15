package geometry

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Pose struct {
	Position Point   `json:"position"`
	Heading  float64 `json:"heading"`
}
