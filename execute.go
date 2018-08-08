package main

import (
    "fmt"
)

type CommandResult struct {
    Cmd *Command
    Err error
}

func ExecuteCommands(in <-chan *Command, out chan<- *CommandResult, notify chan<- int, id int) {
    defer RecoverDo(
        func(x interface{}) {
            notify <- id
            LogDebug("Executor terminates due to:", x)
        },
        func() {
            LogDebug("Executor terminates normally")
        },
    )

    for cmd := range in {
        var e error
        switch cmd.Action {
        case "country": e = SetWifiCountry(cmd.Args[0])
        case "connect": e = WifiConnect(cmd.Args[0], cmd.Args[1])
        case "disconnect": e = WifiDisconnect(cmd.Args[0])
        case "start": e = StartService(cmd.Args[0])
        case "stop": e = StopService(cmd.Args[0])
        case "halt": e = HaltSystem()
        case "reboot": e = RebootSystem()
        default: panic(fmt.Sprintf("Invalid command: %v", cmd))
        }
        out <- &CommandResult{cmd, e}
    }
}

func CommandIsChangingSystemStates(cmd *Command) bool {
    return (cmd.Action == "country" ||
            cmd.Action == "connect" ||
            cmd.Action == "disconnect" ||
            cmd.Action == "start" ||
            cmd.Action == "stop")
}
