package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/Syncano/codebox/pkg/common"
	"github.com/Syncano/codebox/pkg/util"
	brokerpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/broker/v1"
	lbpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/lb/v1"
	scriptpb "github.com/Syncano/syncanoapis/gen/go/syncano/codebox/script/v1"
)

var (
	// ErrMissingPayloadKey means that payload was missing from request headers.
	ErrMissingPayloadKey = errors.New("missing payload")
	// ErrJSONParsingFailed is used to mark any JSON unmarshal error during payload parsing.
	ErrJSONParsingFailed = errors.New("json parsing error, expected an object")
	statusToHTTPCode     = map[string]int{
		failureStatus: http.StatusInternalServerError,
		blockedStatus: http.StatusTooManyRequests,
		timeoutStatus: http.StatusRequestTimeout,
	}
)

const (
	headerInstancePKKey = "INSTANCE_PK"
	headerTracePKKey    = "TRACE_PK"
	headerPayloadKey    = "PAYLOAD_KEY"
	headerPayloadParsed = "PAYLOAD_PARSED"
	getSkipCache        = "__skip_cache"
	jsonContentType     = "application/json; charset=utf-8"
	headerRequestID     = "HTTP_X_REQUEST_ID"
)

type uwsgiPayload struct {
	OutputLimit    uint32            `json:"output_limit"`
	Files          map[string]string `json:"files"`
	SourceHash     string            `json:"source_hash"`
	Entrypoint     string            `json:"entrypoint"`
	Environment    string            `json:"environment"`
	EnvironmentURL string            `json:"environment_url"`
	Trace          json.RawMessage   `json:"trace"`
	Run            uwsgiRunPayload   `json:"run"`
	Cache          float64           `json:"cache"`
	EndpointName   string            `json:"name"`
}

type uwsgiRunPayload struct {
	Args             string  `json:"additional_args"`
	Config           string  `json:"config"`
	Meta             string  `json:"meta"`
	Runtime          string  `json:"runtime_name"`
	ConcurrencyLimit int32   `json:"concurrency_limit"`
	Timeout          float64 `json:"timeout"`
	Async            uint32  `json:"async"`
	MCPU             uint32  `json:"mcpu"`
}

func createCacheKey(schema, endpointName, hash string) string {
	return fmt.Sprintf("%s:cache:s:%s:%s", schema, endpointName, hash)
}

func httpError(w http.ResponseWriter, status int, err string) {
	w.WriteHeader(status)

	if status != 0 {
		w.Header().Set("Content-Type", jsonContentType)
		fmt.Fprintf(w, `{"detail":"%s"}`, err)
	}
}

func writeTraceResponse(w http.ResponseWriter, trace *ScriptTrace) {
	if trace.Result != nil && trace.Result.Response != nil {
		resp := trace.Result.Response
		headers := w.Header()

		for k, v := range resp.Headers {
			headers.Set(k, v)
		}

		headers.Set("Content-Type", resp.ContentType)
		w.WriteHeader(int(resp.Status))
		w.Write(resp.Content) // nolint - ignore error

		return
	}

	// Clear weight before marshaling.
	tmp := trace.Weight
	trace.Weight = 0
	ret, _ := json.Marshal(trace)
	trace.Weight = tmp

	w.Header().Set("Content-Type", jsonContentType)

	httpCode, ok := statusToHTTPCode[trace.Status]
	if ok {
		w.WriteHeader(httpCode)
	}

	w.Write(ret) // nolint - ignore error
}

// RunHandler processes uwsgi request and passes it to load balancer.
func (s *Server) RunHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), s.options.MaxTimeout)
	defer cancel()

	ctx, reqID := util.AddRequestID(ctx, func() string {
		return getRequestID(r)
	})
	logger := logrus.WithFields(logrus.Fields{"peer": r.RemoteAddr, "reqID": reqID})

	if r.Header.Get("HTTP_ORIGIN") != "" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}

	// Process request.
	payload, err := s.loadPayload(r)
	if err != nil {
		logger.WithError(err).Error("Loading payload failed")
		httpError(w, http.StatusInternalServerError, "Loading payload failure.")

		return
	}

	// Process cached response (if it exists).
	cacheKey := createCacheKey(r.Header.Get(headerInstancePKKey), payload.EndpointName, payload.SourceHash)

	if payload.Cache > 0 && r.URL.Query().Get(getSkipCache) != "1" {
		if cacheData, err := s.redisCli.Get(cacheKey).Bytes(); err == nil {
			var trace ScriptTrace
			err := msgpack.Unmarshal(cacheData, &trace)

			if err == nil {
				writeTraceResponse(w, &trace)
				return
			}
		}
	}

	meta, lbMeta, scriptMeta, err := s.prepareRequest(r, payload)
	if err != nil {
		logger.WithError(err).Error("Parsing request failed")
		httpError(w, http.StatusInternalServerError, "Parsing request failure.")

		return
	}

	// Check if request META needs to be parsed.
	requestParsed := r.Header.Get(headerPayloadParsed) == "1"

	// Process script chunks.
	var chunks []*scriptpb.RunChunk

	if !requestParsed {
		r.Body = http.MaxBytesReader(w, r.Body, s.options.MaxPayloadSize)

		chunks, err = s.processRequestData(r)
		if err != nil {
			httpError(w, http.StatusBadRequest, fmt.Sprintf("Parsing payload failure: %s.", err.Error()))
			return
		}
	}

	start := time.Now()
	chunkReader := common.NewArrayChunkReader(chunks)

	stream, err := s.sendRunToLoadbalancer(ctx, logger, meta, lbMeta, scriptMeta, chunkReader)
	if err != nil {
		httpError(w, http.StatusBadGateway, "Processing script failure.")
		return
	}

	trace, _ := s.processResponse(ctx, logger, start, meta, stream, nil)
	took := time.Duration(trace.Duration) * time.Millisecond

	logger.WithFields(logrus.Fields{
		"lbMeta":        lbMeta,
		"runtime":       scriptMeta.Runtime,
		"sourceHash":    scriptMeta.SourceHash,
		"userID":        scriptMeta.UserId,
		"payloadParsed": requestParsed,
		"took":          took,
		"overhead":      time.Since(start) - took,
	}).Info("uwsgi:broker:Run")

	// Process result.
	if payload.Cache > 0 {
		b, err := msgpack.Marshal(trace)
		if err == nil {
			// Save cached trace to redis.
			s.redisCli.Set(cacheKey, b, time.Duration(payload.Cache*1000)*time.Millisecond)
		}
	}

	writeTraceResponse(w, trace)
}

func getRequestID(r *http.Request) string {
	reqID := r.Header.Get(headerRequestID)
	if reqID != "" {
		return reqID
	}

	return util.NewRequestID()
}

func (s *Server) prepareRequest(r *http.Request, payload *uwsgiPayload) (meta *brokerpb.RunMeta, lbMeta *lbpb.RunMeta, scriptMeta *scriptpb.RunMeta, err error) {
	instancePK := r.Header.Get(headerInstancePKKey)

	tracePK, err := strconv.ParseUint(r.Header.Get(headerTracePKKey), 10, 0)
	if err != nil {
		return nil, nil, nil, err
	}

	return &brokerpb.RunMeta{
			Files:          payload.Files,
			EnvironmentUrl: payload.EnvironmentURL,
			Trace:          payload.Trace,
			TraceId:        tracePK,
		},
		&lbpb.RunMeta{
			ConcurrencyKey:   instancePK,
			ConcurrencyLimit: payload.Run.ConcurrencyLimit,
		},
		&scriptpb.RunMeta{
			Environment: payload.Environment,
			Runtime:     payload.Run.Runtime,
			SourceHash:  payload.SourceHash,
			UserId:      instancePK,
			Options: &scriptpb.RunMeta_Options{
				Entrypoint:  payload.Entrypoint,
				OutputLimit: payload.OutputLimit,
				Timeout:     int64(payload.Run.Timeout * 1000),
				Async:       payload.Run.Async,
				Mcpu:        payload.Run.MCPU,
				Args:        []byte(payload.Run.Args),
				Config:      []byte(payload.Run.Config),
				Meta:        []byte(payload.Run.Meta),
			},
		}, nil
}

func (s *Server) loadPayload(r *http.Request) (*uwsgiPayload, error) {
	payloadKey := r.Header.Get(headerPayloadKey)
	if payloadKey == "" {
		return nil, ErrMissingPayloadKey
	}

	payloadBytes, err := s.redisCli.Get(payloadKey).Bytes()
	if err != nil {
		return nil, err
	}

	if err := s.redisCli.Del(payloadKey).Err(); err != nil {
		return nil, err
	}

	var payload uwsgiPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

func (s *Server) processRequestData(r *http.Request) ([]*scriptpb.RunChunk, error) {
	var chunks []*scriptpb.RunChunk

	parseErr := r.ParseMultipartForm(s.options.MaxPayloadSize)

	// Parse GET/POST params.
	dataMap := make(map[string]interface{})

	for k, v := range r.Form {
		if len(k) > 2 && strings.HasSuffix(k, "[]") {
			dataMap[k[:len(k)-2]] = v
		} else {
			dataMap[k] = v[0]
		}
	}

	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		jsonMap := make(map[string]interface{})
		if err := json.NewDecoder(r.Body).Decode(&jsonMap); err != nil {
			return nil, ErrJSONParsingFailed
		}

		for k, v := range jsonMap {
			dataMap[k] = v
		}
	}

	if data, err := json.Marshal(dataMap); err == nil {
		chunks = append(chunks, &scriptpb.RunChunk{
			Data: data,
			Type: scriptpb.RunChunk_ARGS,
		})
	}

	// Process files and append request there.
	if parseErr == nil && r.MultipartForm != nil {
		for name, files := range r.MultipartForm.File {
			file := files[0]

			if f, err := file.Open(); err == nil {
				if buf, e := ioutil.ReadAll(f); e == nil {
					chunks = append(chunks, &scriptpb.RunChunk{
						Name:        name,
						Filename:    file.Filename,
						ContentType: file.Header.Get("Content-Type"),
						Data:        buf,
					})
				}
			}
		}
	}

	return chunks, nil
}
