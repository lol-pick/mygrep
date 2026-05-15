package domain

type Chunk struct {
	ID        string
	Source    string
	StartLine int
	Lines     []string
}
