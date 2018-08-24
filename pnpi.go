package main

import (
    "fmt"
    "flag"
    "strings"
    "io"
    "os/exec"
    "path/filepath"
    "encoding/json"
    "encoding/binary"
    "github.com/google/gousb"
)

func RecoverDo(f func(interface{}), g func()) {
    if r := recover(); r != nil { f(r) } else { g() }
}

func ReadCommands(r io.Reader, out chan<- *Command) {
    defer RecoverDo(
        func(x interface{}) {
            LogDebug("USB Reader terminates due to:", x)
        },
        func() {
            LogDebug("USB Reader terminates normally. This should never happen.")
        },
    )
    defer close(out)

    decoder := json.NewDecoder(r)
    for {
        var cmd Command
        if err := decoder.Decode(&cmd); err != nil {
            panic(fmt.Sprintf("JSON decoder error: %v", err))
        }
        out <- &cmd
    }
}

func WriteReports(ep *gousb.OutEndpoint, in <-chan interface{}, sent chan<- bool, notify chan<- int, id int) {
    defer RecoverDo(
        func(x interface{}) {
            notify <- id
            LogDebug("USB Writer terminates due to:", x)
        },
        func() {
            LogDebug("USB Writer terminates normally")
        },
    )

    for obj := range in {
        var body []byte
        var err error

        if obj == nil {
            if body, err = json.Marshal(struct{}{}); err != nil {
                panic(err)
            }
        } else {
            if body, err = json.Marshal(obj); err != nil {
                panic(err)
            }
        }

        length := len(body)
        if length > 32767 {  // Java short's max value
            LogInfo("USB not writing. Payload too long:", string(body))
            sent <- false
            continue
        }

        LogDebugf("Writing USB Payload (%d bytes): %s", length, string(body))

        header := make([]byte, 2)
        binary.BigEndian.PutUint16(header, uint16(length))

        if _,err = ep.Write(header); err != nil {
            panic(err)
        }

        if _,err = ep.Write(body); err != nil {
            panic(err)
        }

        sent <- true
    }
}

func RetrieveChoices() *SystemChoices {
    countries,_ := AvailableWifiCountries()
    return NewSystemChoices(countries)
}

func Interact(stack *AccessoryModeStack) {
    defer RecoverDo(
        func(x interface{}) {
            LogDebug("Interactor exit due to:", x)
        },
        func() {
            LogDebug("Interactor exit normally")
        },
    )

    usbIn := make(chan *Command)
    go ReadCommands(stack.ReadStream, usbIn)

    // For children to communicate state changes to parent.
    // Right now, the only state change is "Terminate abnormally".
    // I make it a buffered channel in case Interactor is exiting
    // (i.e. stop listening) while a child suffers a panic. A buffered
    // channel allows non-blocking send, even if the other end is not listening.
    notifyIn := make(chan int, 9)
    const (
        usbWriterId = 1 << iota
        monitorId
        executorId
        scannerId
    )

    usbOut, sentIn := make(chan interface{}, 9), make(chan bool)
    go WriteReports(stack.OutEndpoint, usbOut, sentIn, notifyIn, usbWriterId)
    defer close(usbOut)  // terminate writer
    usbWriterLive := true
    usbWriterPending := 0
    const USB_WRITER_PENDING_MAX = 3
    // Considerations affecting USB_WRITER_PENDING_MAX value:
    // - smaller than usbOut channel buffer size: We want to ensure putting
    //   things into channel won't block.
    // - terminate Interact() function reasonably quickly to check for accessory
    //   again: USB writer blockage usually results from Android app crashing
    //   or stream closed inadvertently on the app side. We can expect user to
    //   open the app or re-plug USB very soon.

    monitorControlOut, monitorReportsIn := make(chan int, 9), make(chan *MonitorReport)
    go MonitorSystem(monitorControlOut, monitorReportsIn, notifyIn, monitorId)
    defer close(monitorControlOut)  // terminate monitor
    monitorLive := true

    commandsOut, commandResultsIn := make(chan *Command, 9), make(chan *CommandResult)
    go ExecuteCommands(commandsOut, commandResultsIn, notifyIn, executorId)
    defer close(commandsOut)  // terminate executor
    executorLive := true

    scannerControlOut, scanResultsIn := make(chan int, 9), make(chan *ScanResult)
    go WifiScan(scannerControlOut, scanResultsIn, notifyIn, scannerId)
    defer close(scannerControlOut)
    scannerLive := true

    choicesRetrieved := false

    for {
        select {
        case command, ok := <-usbIn:
            if !ok {
                LogDebug("USB Reader died. I am dying too.")
                return
            }

            LogDebugf("USB command received: %v", command)

            switch command.Action {
            case "monitor":
                if monitorLive {
                    switch command.Args[0] {
                    case "start":
                        if !choicesRetrieved {
                            usbOut <- RetrieveChoices()
                            choicesRetrieved = true
                        }
                        monitorControlOut <- MonitorStart

                    case "stop":  monitorControlOut <- MonitorStop
                    }
                }
            case "scan":
                if scannerLive {
                    switch command.Args[0] {
                    case "start": scannerControlOut <- ScanStart
                    case "stop":  scannerControlOut <- ScanStop
                    }
                }

            case "exit":
                return

            default:
                if executorLive {
                    commandsOut <- command

                    if CommandIsChangingSystemStates(command) {
                        if monitorLive { monitorControlOut <- MonitorBurst }
                    }
                }
            }

        case commandResult := <-commandResultsIn:
            LogDebugf("Executor result received: %v", commandResult)

        case monitorReport := <-monitorReportsIn:
            LogDebugf("Monitor report received: %v", monitorReport)
            if usbWriterLive {
                if usbWriterPending > USB_WRITER_PENDING_MAX {
                    LogInfof(
                        "USB pending-counter exceeds %d, writer seems blocked, I am dying.",
                        USB_WRITER_PENDING_MAX)
                    return
                }

                if monitorReport == nil {
                    usbOut <- nil
                    usbWriterPending++
                } else if monitorReport.Full {
                    usbOut <- NewSystemStates(
                                    monitorReport.Interfaces,
                                    monitorReport.Services,
                                    monitorReport.WifiCountryCode)
                    usbWriterPending++
                } else {
                    usbOut <- NewSystemStatesChange(
                                    monitorReport.Interfaces,
                                    monitorReport.Services,
                                    monitorReport.WifiCountryCode)
                    usbWriterPending++
                }
            }

        case <-sentIn:
            usbWriterPending--

        case scanResult := <-scanResultsIn:
            LogDebugf("Scan result received: %v", scanResult)
            if usbWriterLive {
                if usbWriterPending > USB_WRITER_PENDING_MAX {
                    LogInfof(
                        "USB pending-counter exceeds %d, writer seems blocked, I am dying.",
                        USB_WRITER_PENDING_MAX)
                    return
                }

                if scanResult != nil {
                    usbOut <- scanResult
                    usbWriterPending++
                }
            }

        case child := <-notifyIn:
            switch (child) {
            case usbWriterId:
                usbWriterLive = false
                LogDebug("USB Writer died")
            case monitorId:
                monitorLive = false
                LogDebug("Monitor died")
            case executorId:
                executorLive = false
                LogDebug("Executor died")
            case scannerId:
                scannerLive = false
                LogDebug("Scanner died")
            }
        }
    }
}

const (
    ServerVersion = "2.1"
)

func Init() bool {
    scriptDirectory := flag.String("d", "", "Helper script directory")
    lessOutput := flag.Bool("z", false, "Less output")
    printVersion := flag.Bool("version", false, "Print version number and exit")

    flag.Parse()

    if (*printVersion) {
        fmt.Println("Plug n Pi Server Version:", ServerVersion)
        fmt.Println("Protocol Version:", AoaProtocolVersion)
        return false
    }

    if (*lessOutput) {
        SetLogLevel(Info)
    } else {
        SetLogLevel(Debug)
    }

    if (*scriptDirectory == "") {
        fmt.Println("No specified helper script directory. Use -d to specify.")
        return false
    }

    dir, err := filepath.Abs(*scriptDirectory)
    if err != nil {
        LogFatal(err)
    }
    SetScriptDirectory(dir)

    return true
}

func CheckRunning() {
    out, err := exec.Command("pgrep", "--exact", "pnpi").Output()
    if err == nil {
        lines := strings.Split(strings.TrimSpace(string(out)), "\n")
        if len(lines) > 1 {
            LogFatal("pnpi is already running")
        }
    }
}

func Check() {
    CheckScript()
    CheckRunning()
}

func main() {
    if !Init() { return }
    Check()
    for {
        func() {
            s := OpenAccessoryModeStack()
            defer s.Close()

            Interact(s)
        }()
    }
}
