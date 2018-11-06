package celery

import (
	"encoding/json"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"

	"github.com/Syncano/codebox/pkg/celery/mocks"
)

func TestCelery(t *testing.T) {
	amqpCh := new(mocks.AMQPChannel)
	queue := "queue"
	Init(amqpCh)

	Convey("NewTask works with nil args and kwargs", t, func() {
		task := NewTask("sometask", queue, nil, nil)
		So(len(task.Args), ShouldEqual, 0)
		So(len(task.Kwargs), ShouldEqual, 0)

		Convey("Publish publishes task to amqp", func() {
			amqpCh.On("Publish", mock.Anything, queue, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			e := task.Publish()
			So(e, ShouldBeNil)
			amqpCh.AssertExpectations(t)
		})
		Convey("Publish propagates json marshal error", func() {
			task.Args = []interface{}{json.RawMessage([]byte("a"))}
			e := task.Publish()
			So(e, ShouldNotBeNil)
			amqpCh.AssertExpectations(t)
		})
	})
}
