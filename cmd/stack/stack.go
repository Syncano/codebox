package stack

// This is a slightly improved version of https://github.com/Gurpartap/logrus-stack

import (
	"strings"

	"github.com/facebookgo/stack"
	"github.com/sirupsen/logrus"
)

// NewHook is the initializer for LogrusStackHook{} (implementing logrus.Hook).
// Set levels to callerLevels for which "caller" value may be set, providing a
// single frame of stack. Set levels to stackLevels for which "stack" value may
// be set, providing the full stack (minus logrus).
func NewHook(inAppPrefix string, callerLevels []logrus.Level, stackLevels []logrus.Level) *LogrusStackHook {
	return &LogrusStackHook{
		InAppPrefix:  inAppPrefix,
		CallerLevels: callerLevels,
		StackLevels:  stackLevels,
	}
}

// LogrusStackHook is an implementation of logrus.Hook interface.
type LogrusStackHook struct {
	// Set levels to CallerLevels for which "caller" value may be set,
	// providing a single frame of stack.
	InAppPrefix  string
	CallerLevels []logrus.Level

	// Set levels to StackLevels for which "stack" value may be set,
	// providing the full stack (minus logrus).
	StackLevels []logrus.Level
}

// Levels provides the levels to filter.
func (hook *LogrusStackHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func stripInAppPrefix(prefix, file string) string {
	prefixI := strings.LastIndex(file, prefix)
	if prefixI == -1 {
		prefixI = 0
	} else {
		prefixI += len(prefix) + 1
	}
	return file[prefixI:]
}

// Fire is called by logrus when something is logged.
func (hook *LogrusStackHook) Fire(entry *logrus.Entry) error {
	delete(entry.Data, "stack")
	delete(entry.Data, "caller")

	var skipFrames int
	if len(entry.Data) == 0 {
		// When WithField(s) is not used, we have 7 logrus frames to skip.
		skipFrames = 7
	} else {
		// When WithField(s) is used, we have 5 logrus frames to skip.
		skipFrames = 5
	}

	for _, level := range hook.CallerLevels {
		if entry.Level == level {
			caller := stack.Caller(skipFrames)
			caller.File = stripInAppPrefix(hook.InAppPrefix, caller.File)
			entry.Data["caller"] = caller
			break
		}
	}
	for _, level := range hook.StackLevels {
		if entry.Level == level {
			entry.Data["stack"] = stack.Callers(skipFrames)
			break
		}
	}
	return nil
}
