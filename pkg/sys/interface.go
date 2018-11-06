package sys

// SystemChecker provides methods for getting info about the system.
//go:generate mockery -name SystemChecker
type SystemChecker interface {
	Reset()
	GetMemory() (uint64, uint64)
	CheckFreeMemory(want uint64) error
	ReserveMemory(amount uint64) error
	FreeMemory(amount uint64)
	AvailableMemory() uint64

	GetDiskUsage(path string) (float64, uint64)
}

// Assert that SigarCheck is compatible with our interface.
var _ SystemChecker = (*SigarChecker)(nil)
