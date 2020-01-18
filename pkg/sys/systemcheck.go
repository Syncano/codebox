package sys

import (
	"fmt"
	"sync"

	sigar "github.com/cloudfoundry/gosigar"

	"github.com/Syncano/codebox/pkg/util"
)

// SigarChecker defines a struct that is used to get information about the host system.
type SigarChecker struct {
	mu             sync.RWMutex
	reservedMemory uint64
}

// ErrNotEnoughMemory signals that there is not enough memory available.
type ErrNotEnoughMemory struct {
	Want     uint64
	Free     uint64
	Reserved uint64
}

func (e ErrNotEnoughMemory) Error() string {
	return fmt.Sprintf("not enough memory {Want:%d, Free:%d, Reserved:%d}", e.Want, e.Free, e.Reserved)
}

// Reset resets internal state.
func (s *SigarChecker) Reset() {
	s.mu.Lock()
	s.reservedMemory = 0
	s.mu.Unlock()
}

// GetMemory returns info about memory.
func (s *SigarChecker) GetMemory() (total, actualFree uint64) {
	var mem sigar.Mem
	err := mem.Get()
	util.Must(err)

	return mem.Total, mem.ActualFree
}

// CheckFreeMemory checks if there is enough actual free memory available.
func (s *SigarChecker) CheckFreeMemory(want uint64) error {
	_, actualFree := s.GetMemory()
	if actualFree < want {
		return ErrNotEnoughMemory{Reserved: s.reservedMemory, Free: actualFree, Want: want}
	}

	return nil
}

// ReserveMemory locks (reserves) specified amount of memory (internally).
func (s *SigarChecker) ReserveMemory(want uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, actualFree := s.GetMemory()
	if actualFree < s.reservedMemory+want {
		return ErrNotEnoughMemory{Reserved: s.reservedMemory, Free: actualFree, Want: want}
	}

	s.reservedMemory += want

	return nil
}

// AvailableMemory returns free - reserved memory.
func (s *SigarChecker) AvailableMemory() uint64 {
	_, actualFree := s.GetMemory()
	s.mu.RLock()
	reserved := s.reservedMemory
	s.mu.RUnlock()

	return actualFree - reserved
}

// FreeMemory unlocks specified amount of reserved memory.
func (s *SigarChecker) FreeMemory(amount uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reservedMemory -= amount
}

// GetDiskUsage returns info about disk usage at path.
func (s *SigarChecker) GetDiskUsage(path string) (usePercent float64, avail uint64) {
	var fsu sigar.FileSystemUsage

	err := fsu.Get(path)

	util.Must(err)

	return fsu.UsePercent(), fsu.Avail
}
