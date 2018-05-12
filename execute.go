package main

import (
    "fmt"
    "log"
    "os/exec"
)

func wifiConnect(ssid string, passphrase string) error {
    _,err := raspi_config_nonint("do_wifi_ssid_passphrase", ssid, passphrase)
    return err
}

func startService(name string) error {
    _,err := raspi_config_nonint(service_fn(name), "0")
    return err
}

func stopService(name string) error {
    _,err := raspi_config_nonint(service_fn(name), "1")
    return err
}

func haltSystem() error {
    return exec.Command("halt", "-h").Run()
}

func rebootSystem() error {
    return exec.Command("reboot").Run()
}

type CommandResult struct {
    Cmd *Command
    Err error
}

func ExecuteCommands(in <-chan *Command, out chan<- *CommandResult, notify chan<- int, id int) {
    defer RecoverDo(
        func(x interface{}) {
            notify <- id
            log.Println("Executor terminates due to:", x)
        },
        func() {
            log.Println("Executor terminates normally")
        },
    )

    for cmd := range in {
        var e error
        switch cmd.Action {
        case "connect": e = wifiConnect(cmd.Args[0], cmd.Args[1])
        case "start": e = startService(cmd.Args[0])
        case "stop": e = stopService(cmd.Args[0])
        case "halt": e = haltSystem()
        case "reboot": e = rebootSystem()
        default: panic(fmt.Sprintf("Invalid command: %v", cmd))
        }
        out <- &CommandResult{cmd, e}
    }
}

func CommandIsChangingSystemStates(cmd *Command) bool {
    return (cmd.Action == "connect" ||
            cmd.Action == "start" ||
            cmd.Action == "stop")
}
