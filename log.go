package main

import "log"

const (
    Debug = iota
    Info
)

var logLevel = Debug

func SetLogLevel(level int) {
    logLevel = level
}

func LogFatal(v ...interface{}) {
    log.Fatal(v...)
}

func LogFatalf(format string, v ...interface{}) {
    log.Fatalf(format, v...)
}

func LogDebug(v ...interface{}) {
    if logLevel <= Debug {
        log.Println(v...)
    }
}

func LogDebugf(format string, v ...interface{}) {
    if logLevel <= Debug {
        log.Printf(format, v...)
    }
}

func LogInfo(v ...interface{}) {
    if logLevel <= Info {
        log.Println(v...)
    }
}

func LogInfof(format string, v ...interface{}) {
    if logLevel <= Info {
        log.Printf(format, v...)
    }
}
