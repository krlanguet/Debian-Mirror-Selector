package logger

import (
    "os"
    "io"
    "io/ioutil"
    "log"
    "github.com/davecgh/go-spew/spew"
)

type Logger struct {
    *log.Logger
    out io.Writer
}

func (l *Logger) Dump(a ...interface{}) {
    spew.Fdump(l.out, a...)
}

func New(logOn bool) Logger {
    var out io.Writer
    if logOn {
        out = os.Stdout
    } else {
        out = ioutil.Discard
    }
    return Logger{
        Logger: log.New(out, "", log.LstdFlags),
        out: out,
    }
}

