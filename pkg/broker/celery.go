package broker

import (
	"encoding/json"
	"time"

	"github.com/Syncano/codebox/pkg/celery"
	scriptpb "github.com/Syncano/codebox/pkg/script/proto"
	"github.com/Syncano/codebox/pkg/util"
)

var (
	codeToStatus = map[int32]string{
		0:   successStatus,
		124: timeoutStatus,
	}
)

const (
	codeboxQueue          = "codebox"
	celeryUpdateTraceTask = "apps.codeboxes.tasks.UpdateTraceTask"
	celerySaveTraceTask   = "apps.codeboxes.tasks.SaveTraceTask"
	successStatus         = "success"
	errorStatus           = "error"
	blockedStatus         = "blocked"
	failureStatus         = "failure"
	timeoutStatus         = "timeout"
)

// ScriptTrace defines a serialized form of a script trace.
type ScriptTrace struct {
	ID         uint64             `json:"id,omitempty"`
	ExecutedAt string             `json:"executed_at"`
	Status     string             `json:"status"`
	Duration   int64              `json:"duration"`
	Result     *ScriptTraceResult `json:"result,omitempty"`
	Weight     uint32             `json:"weight,omitempty"`
}

// ScriptTraceResult defines a serialized form of script result (nested in script trace).
type ScriptTraceResult struct {
	Stdout   json.RawMessage            `json:"stdout"`
	Stderr   json.RawMessage            `json:"stderr"`
	Response *ScriptTraceResultResponse `json:"response,omitempty"`
}

// ScriptTraceResultResponse defines a serialized form of script result response (nested in script trace result).
type ScriptTraceResultResponse struct {
	Status      int32             `json:"status"`
	ContentType string            `json:"content_type"`
	Content     []byte            `json:"-"`
	Headers     map[string]string `json:"headers"`
}

// NewScriptTrace creates a new script trace from a given script result.
func NewScriptTrace(traceID uint64, result *scriptpb.RunResponse) *ScriptTrace {
	status := blockedStatus

	var (
		scriptResult *ScriptTraceResult
		executed     time.Time
	)

	if result != nil {
		var ok bool

		status, ok = codeToStatus[result.GetCode()]
		if !ok {
			status = failureStatus
		}

		// Parse script response for celery JSON object.
		var scriptResponse *ScriptTraceResultResponse

		response := result.GetResponse()
		if response != nil {
			scriptResponse = &ScriptTraceResultResponse{
				Status:      response.StatusCode,
				ContentType: response.ContentType,
				Content:     response.Content,
				Headers:     response.Headers,
			}
		}

		// Parse script response result for celery JSON object.
		scriptResult = &ScriptTraceResult{
			Stdout:   json.RawMessage(util.ToQuoteJSON(result.GetStdout())),
			Stderr:   json.RawMessage(util.ToQuoteJSON(result.GetStderr())),
			Response: scriptResponse,
		}
		executed = time.Unix(0, result.GetTime())
	} else {
		executed = time.Now()
	}

	return &ScriptTrace{
		ID:         traceID,
		ExecutedAt: executed.Format("2006-01-02T15:04:05.999999Z"),
		Status:     status,
		Duration:   result.GetTook(),
		Result:     scriptResult,
		Weight:     result.GetWeight(),
	}
}

// NewCelerySaveTask returns a new save trace task object.
func NewCelerySaveTask(trace []byte, updatedTrace *ScriptTrace) *celery.Task {
	return celery.NewTask(celerySaveTraceTask, codeboxQueue, []interface{}{
		json.RawMessage(trace),
		updatedTrace,
	}, nil)
}

// NewCeleryUpdateTask returns a new update trace task object.
func NewCeleryUpdateTask(trace []byte) *celery.Task {
	return celery.NewTask(celeryUpdateTraceTask, codeboxQueue, []interface{}{
		json.RawMessage(trace),
	}, nil)
}
