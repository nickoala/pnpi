package main

import (
    "fmt"
    "os/exec"
    "regexp"
    "strconv"
    "time"
)

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
        LogDebug("iwlist failed:", err)
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
    return NewScanResult(hotspots)
}

const (
    ScanStart = 1 << iota
    ScanStop
)

func WifiScan(in <-chan int, out chan<- *ScanResult, notify chan<- int, id int) {
    defer RecoverDo(
        func(x interface{}) {
            notify <- id
            LogDebug("Scanner terminates due to:", x)
        },
        func() {
            LogDebug("Scanner terminates normally")
        },
    )

    // `iwlist scan` can take 5 seconds. I give it some margin.
    ticker := time.NewTicker(6600 * time.Millisecond)
    defer ticker.Stop()
    active := false
    cool := 0

    filter := func(r *ScanResult) {
        if len(r.Hotspots) > 0 {
            cool = 0
            out <- r
        } else {
            // `iwlist scan` occasionally fails to see hotspots.
            // Send empty result only if seeing no hotspots twice in a row.
            cool++
            if cool >= 2 {
                cool = 2
                out <- r
            }
        }
    }

    for {
        select {
        case ctrl, ok := <-in:
            if !ok { return }

            switch ctrl {
            case ScanStart:
                active = true
                filter(scanForResult())
            case ScanStop:
                active = false
            default:
                panic(fmt.Sprintf("Invalid scan control code: %v", ctrl))
            }
        case <-ticker.C:
            if active {
                filter(scanForResult())
            }
        }
    }
}
