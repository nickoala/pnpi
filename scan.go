package main

import (
    "fmt"
    "log"
    "os/exec"
    "regexp"
    "strconv"
    "time"
)

type ScanResult struct {
    Hotspots []Hotspot
}

var hexPattern = regexp.MustCompile(`\\x[0-9a-fA-F]{2}`)

func ssidIsValid(ssid string) bool {
    if ssid == "" { return false }

    // Filter out "\x00\x00\x00...\x00\x00\x00"
    if len(hexPattern.FindAllString(ssid, -1)) >= 6 { return false }

    return true
}

var infoPattern = regexp.MustCompile(`(?ms)Signal level=(-[0-9]+) dBm.*?Encryption key:(on|off).*?ESSID:"(.*?)"`)

func scanForResult() *ScanResult {
    out, err := exec.Command("iwlist", "scan").Output()
    if err != nil {
        log.Println("iwlist failed:", err)
        return nil
    }

    sections := infoPattern.FindAllStringSubmatch(string(out), -1)

    hotspots := make([]Hotspot, 0)  // ensure not nil
    for _,s := range sections {
        signalLevel, encryption, ssid := s[1], s[2], s[3]
        if !ssidIsValid(ssid) { continue }

        signal, err := strconv.Atoi(signalLevel)
        if err != nil { continue }

        hotspots = append(hotspots, Hotspot{ssid, encryption=="off", signal})
    }
    return &ScanResult{hotspots}
}

const (
    ScanStart = 1 << iota
    ScanStop
)

func WifiScan(in <-chan int, out chan<- *ScanResult, notify chan<- int, id int) {
    defer RecoverDo(
        func(x interface{}) {
            notify <- id
            log.Println("Scanner terminates due to:", x)
        },
        func() {
            log.Println("Scanner terminates normally")
        },
    )

    // `iwlist scan` can take 5 seconds. I give it some margin.
    ticker := time.NewTicker(6600 * time.Millisecond)
    defer ticker.Stop()
    active := false

    for {
        select {
        case ctrl, ok := <-in:
            if !ok { return }

            switch ctrl {
            case ScanStart:
                active = true
                out <- scanForResult()
            case ScanStop:
                active = false
            default:
                panic(fmt.Sprintf("Invalid scan control code: %v", ctrl))
            }
        case <-ticker.C:
            if active {
                out <- scanForResult()
            }
        }
    }
}
