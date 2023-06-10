package log

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2/utils"
)

const work = "work"

func initDefaultLogger() {
	logger = &defaultLogger{
		stdlog: log.New(os.Stderr, "", 0),
		depth:  4,
	}
}

type byteSliceWriter struct {
	b []byte
}

func (w *byteSliceWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

func Test_DefaultLogger(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	Trace("trace work")
	Debug("received work order")
	Info("starting work")
	Warn("work may fail")
	Error("work failed")
	Panic("work panic")
	utils.AssertEqual(t, "[Trace] trace work\n"+
		"[Debug] received work order\n"+
		"[Info] starting work\n"+
		"[Warn] work may fail\n"+
		"[Error] work failed\n"+
		"[Panic] work panic\n", string(w.b))
}

func Test_DefaultFormatLogger(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	Tracef("trace %s", work)
	Debugf("received %s order", work)
	Infof("starting %s", work)
	Warnf("%s may fail", work)
	Errorf("%s failed", work)
	Panicf("%s panic", work)

	utils.AssertEqual(t, "[Trace] trace work\n"+
		"[Debug] received work order\n"+
		"[Info] starting work\n"+
		"[Warn] work may fail\n"+
		"[Error] work failed\n"+
		"[Panic] work panic\n", string(w.b))
}

func Test_CtxLogger(t *testing.T) {
	initDefaultLogger()

	var w byteSliceWriter
	SetOutput(&w)

	ctx := context.Background()

	CtxTracef(ctx, "trace %s", work)
	CtxDebugf(ctx, "received %s order", work)
	CtxInfof(ctx, "starting %s", work)
	CtxWarnf(ctx, "%s may fail", work)
	CtxErrorf(ctx, "%s failed", work)
	CtxPanicf(ctx, "%s panic", work)

	utils.AssertEqual(t, "[Trace] trace work\n"+
		"[Debug] received work order\n"+
		"[Info] starting work\n"+
		"[Warn] work may fail\n"+
		"[Error] work failed\n"+
		"[Panic] work panic\n", string(w.b))
}

func Test_SetLevel(t *testing.T) {
	setLogger := &defaultLogger{
		stdlog: log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
		depth:  4,
	}

	setLogger.SetLevel(LevelTrace)
	utils.AssertEqual(t, LevelTrace, setLogger.level)
	utils.AssertEqual(t, LevelTrace.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelDebug)
	utils.AssertEqual(t, LevelDebug, setLogger.level)
	utils.AssertEqual(t, LevelDebug.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelInfo)
	utils.AssertEqual(t, LevelInfo, setLogger.level)
	utils.AssertEqual(t, LevelInfo.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelWarn)
	utils.AssertEqual(t, LevelWarn, setLogger.level)
	utils.AssertEqual(t, LevelWarn.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelError)
	utils.AssertEqual(t, LevelError, setLogger.level)
	utils.AssertEqual(t, LevelError.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelFatal)
	utils.AssertEqual(t, LevelFatal, setLogger.level)
	utils.AssertEqual(t, LevelFatal.toString(), setLogger.level.toString())

	setLogger.SetLevel(LevelPanic)
	utils.AssertEqual(t, LevelPanic, setLogger.level)
	utils.AssertEqual(t, LevelPanic.toString(), setLogger.level.toString())

	setLogger.SetLevel(8)
	utils.AssertEqual(t, 8, int(setLogger.level))
	utils.AssertEqual(t, "[?8] ", setLogger.level.toString())
}
