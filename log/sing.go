package log

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"

	L "github.com/sagernet/sing/common/logger"
)

type singLogger struct{}

func (l singLogger) TraceContext(ctx context.Context, args ...any) {
	l.Debug(args...)
}

func (l singLogger) DebugContext(ctx context.Context, args ...any) {
	l.Debug(args...)
}

func (l singLogger) InfoContext(ctx context.Context, args ...any) {
	l.Info(args...)
}

func (l singLogger) WarnContext(ctx context.Context, args ...any) {
	l.Warn(args...)
}

func (l singLogger) ErrorContext(ctx context.Context, args ...any) {
	l.Error(args...)
}

func (l singLogger) FatalContext(ctx context.Context, args ...any) {
	l.Fatal(args...)
}

func (l singLogger) PanicContext(ctx context.Context, args ...any) {
	l.Panic(args...)
}

func (l singLogger) Trace(args ...any) {
	event := singLog(DEBUG, args...)
	logCh <- event
	print(event)
}

func (l singLogger) Debug(args ...any) {
	event := singLog(DEBUG, args...)
	logCh <- event
	print(event)
}

func (l singLogger) Info(args ...any) {
	event := singLog(INFO, args...)
	logCh <- event
	print(event)
	Infoln(fmt.Sprint(args...))
}

func (l singLogger) Warn(args ...any) {
	event := singLog(WARNING, args...)
	logCh <- event
	print(event)
	Warnln(fmt.Sprint(args...))
}

func (l singLogger) Error(args ...any) {
	event := singLog(ERROR, args...)
	logCh <- event
	print(event)
	Errorln(fmt.Sprint(args...))
}

func (l singLogger) Fatal(args ...any) {
	log.Fatalln(fmt.Sprint(args...))
}

func (l singLogger) Panic(args ...any) {
	log.Fatalln(fmt.Sprint(args...))
}

func singLog(logLevel LogLevel, args ...any) Event {
	return Event{
		LogLevel: logLevel,
		Payload:  fmt.Sprint(args...),
	}
}

var SingLogger L.ContextLogger = singLogger{}
