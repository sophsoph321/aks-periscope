package interfaces

// Diagnoser defines interface for a diagnoser
type Diagnoser interface {
	GetName() string

	GetFiles() []string

	Diagnose() error

	Export() error
}
