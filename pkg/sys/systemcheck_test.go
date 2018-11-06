package sys

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSystemCheck(t *testing.T) {
	Convey("Given empty sigarcheck struct", t, func() {
		sc := new(SigarChecker)

		Convey("GetMemory returns free memory", func() {
			total, free := sc.GetMemory()
			So(total, ShouldBeGreaterThan, 0)
			So(free, ShouldBeGreaterThan, 0)
			So(total, ShouldBeGreaterThan, free)
		})

		Convey("CheckFreeMemory returns nil if satisfied", func() {
			e := sc.CheckFreeMemory(10)
			So(e, ShouldBeNil)
		})
		Convey("CheckFreeMemory returns error if too high value", func() {
			e := sc.CheckFreeMemory(math.MaxUint64)
			_, ok := e.(ErrNotEnoughMemory)
			So(ok, ShouldBeTrue)
			So(e.Error(), ShouldContainSubstring, "not enough memory")
		})

		Convey("ReserveMemory reserves memory in struct when satisfied", func() {
			e := sc.ReserveMemory(25 * 1024 * 1024)
			So(e, ShouldBeNil)
			So(sc.reservedMemory, ShouldEqual, 25*1024*1024)

			Convey("and FreeMemory frees it", func() {
				sc.FreeMemory(10 * 1024 * 1024)
				So(sc.reservedMemory, ShouldEqual, 15*1024*1024)
			})
			Convey("and AvailableMemory returns free - reserved", func() {
				_, free := sc.GetMemory()
				So(sc.AvailableMemory(), ShouldBeLessThan, free)
			})
			Convey("and Reset resets it", func() {
				sc.Reset()
				So(sc.reservedMemory, ShouldEqual, 0)
			})
		})
		Convey("ReserveMemory returns error if too high value", func() {
			e := sc.ReserveMemory(math.MaxUint64)
			_, ok := e.(ErrNotEnoughMemory)
			So(ok, ShouldBeTrue)
		})

		Convey("FreeMemory returns free disk", func() {
			percent, free := sc.GetDiskUsage("/")
			So(percent, ShouldBeBetween, 0, 100)
			So(free, ShouldBeGreaterThan, 0)
		})

		Convey("GetDiskUsage returns free disk", func() {
			percent, free := sc.GetDiskUsage("/")
			So(percent, ShouldBeBetween, 0, 100)
			So(free, ShouldBeGreaterThan, 0)
		})
	})
}
