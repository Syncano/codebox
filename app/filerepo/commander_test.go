package filerepo_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	. "github.com/Syncano/codebox/app/filerepo"
)

func TestCommander(t *testing.T) {
	Convey("Run executes command", t, func() {
		cmd := new(Command)
		err := cmd.Run("echo", "1")
		So(err, ShouldBeNil)
	})
}
