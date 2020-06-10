package broker

import (
	"encoding/json"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

func TestCelery(t *testing.T) {
	Convey("NewScriptTrace works with utf8", t, func() {
		trace := NewScriptTrace(1, &scriptpb.RunResponse{
			Code:   0,
			Time:   nil,
			Stdout: []byte("zażółć"),
			Stderr: []byte("zażółć"),
		})
		So(trace.Result.Stdout, ShouldResemble, json.RawMessage(`"za\u017c\u00f3\u0142\u0107"`))
		So(trace.Result.Stderr, ShouldResemble, json.RawMessage(`"za\u017c\u00f3\u0142\u0107"`))
	})
}
