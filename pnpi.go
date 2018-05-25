package main

import (
    "fmt"
    "log"
    "io"
    "os"
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
            log.Println("USB Reader terminates due to:", x)
        },
        func() {
            log.Println("USB Reader terminates normally. This should never happen.")
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

func WriteReports(ep *gousb.OutEndpoint, in <-chan *Report, sent chan<- bool, notify chan<- int, id int) {
    defer RecoverDo(
        func(x interface{}) {
            notify <- id
            log.Println("USB Writer terminates due to:", x)
        },
        func() {
            log.Println("USB Writer terminates normally")
        },
    )

    for rpt := range in {
        var body []byte
        var err error

        if rpt == nil {
            if body, err = json.Marshal(struct{}{}); err != nil {
                panic(err)
            }
        } else {
            if body, err = json.Marshal(rpt); err != nil {
                panic(err)
            }
        }

        length := len(body)
        if length > 32767 {  // Java short's max value
            log.Println("USB not writing. Payload too long:", string(body))
            sent <- false
            continue
        }

        log.Printf("Writing USB Payload (%d bytes): %s", length, string(body))

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

func Interact(stack *AccessoryModeStack) {
    defer RecoverDo(
        func(x interface{}) {
            log.Println("Interactor exit due to:", x)
        },
        func() {
            log.Println("Interactor exit normally")
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

    usbOut, sentIn := make(chan *Report, 9), make(chan bool)
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

    monitorControlOut, systemReportsIn := make(chan int, 9), make(chan *SystemReport)
    go MonitorSystemStates(monitorControlOut, systemReportsIn, notifyIn, monitorId)
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

    for {
        select {
        case command, ok := <-usbIn:
            if !ok {
                log.Println("USB Reader died. I am dying too.")
                return
            }

            log.Printf("USB command received: %v", command)

            switch command.Action {
            case "system":
                if monitorLive {
                    switch command.Args[0] {
                    case "start": monitorControlOut <- MonitorStart
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
            log.Printf("Executor result received: %v", commandResult)

        case systemReport := <-systemReportsIn:
            log.Printf("Monitor report received: %v", systemReport)
            if usbWriterLive {
                if usbWriterPending > USB_WRITER_PENDING_MAX {
                    log.Printf(
                        "USB pending-counter exceeds %d, writer seems blocked, I am dying.",
                        USB_WRITER_PENDING_MAX)
                    return
                }

                if systemReport == nil {
                    usbOut <- nil
                    usbWriterPending++
                } else if systemReport.Full {
                    usbOut <- &Report{"system",
                                    systemReport.Interfaces,
                                    systemReport.Services,
                                    nil}
                    usbWriterPending++
                } else {
                    usbOut <- &Report{"change",
                                    systemReport.Interfaces,
                                    systemReport.Services,
                                    nil}
                    usbWriterPending++
                }
            }

        case <-sentIn:
            usbWriterPending--

        case scanResult := <-scanResultsIn:
            log.Printf("Scan result received: %v", scanResult)
            if usbWriterLive {
                if usbWriterPending > USB_WRITER_PENDING_MAX {
                    log.Printf(
                        "USB pending-counter exceeds %d, writer seems blocked, I am dying.",
                        USB_WRITER_PENDING_MAX)
                    return
                }

                if scanResult != nil {
                    usbOut <- &Report{"scan", nil, nil, scanResult.Hotspots}
                    usbWriterPending++
                }
            }

        case child := <-notifyIn:
            switch (child) {
            case usbWriterId:
                usbWriterLive = false
                log.Println("USB Writer died")
            case monitorId:
                monitorLive = false
                log.Println("Monitor died")
            case executorId:
                executorLive = false
                log.Println("Executor died")
            case scannerId:
                scannerLive = false
                log.Println("Scanner died")
            }
        }
    }
}

func main() {
    dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
    if err != nil {
        log.Fatal(err)
    }
    ScriptDirectory = dir

    // Check raspi-config present
    raspiconfigLocation := filepath.Join(ScriptDirectory, "raspi-config")
    info, err := os.Stat(raspiconfigLocation)
    if err != nil {
        log.Fatalf("%s not found: %v", raspiconfigLocation, err)
    }

    // Check executable bits set
    mode := info.Mode()
    if (mode & 0x49 != 0x49) {  // 0x49 == 001001001
        log.Fatalf("%s must be always executable, e.g. rwxr-xr-x", raspiconfigLocation)
    }

    for {
        func() {
            s := OpenAccessoryModeStack()
            defer s.Close()

            Interact(s)
        }()
    }
}
