package interfaces

// Collector defines interface for a collector
type Collector interface {
	GetName() string

	GetFiles() []string

	Collect() error

	Export() error
}
