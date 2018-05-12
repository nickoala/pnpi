package main

import (
    "fmt"
    "log"
    "strings"
    "net"
    "os/exec"
    "time"
)

type NetworkInterfaceMap map[string]NetworkInterface
type ServiceMap map[string]Service

type SystemStates struct {
    Interfaces NetworkInterfaceMap
    Services ServiceMap
}

func (m NetworkInterfaceMap) Keys() *StringSet {
    ss := NewStringSet()
	  for k := range m {
	      ss.Add(k)
	  }
    return ss
}

func (m ServiceMap) Keys() *StringSet {
    ss := NewStringSet()
	  for k := range m {
	      ss.Add(k)
	  }
    return ss
}

func (m NetworkInterfaceMap) Values() []NetworkInterface {
    i, ns := 0, make([]NetworkInterface, len(m))
    for _,n := range m {
        ns[i] = n
        i++
    }
    return ns
}

func (m ServiceMap) Values() []Service {
    i, ss := 0, make([]Service, len(m))
    for _,s := range m {
        ss[i] = s
        i++
    }
    return ss
}

func findSSID(name string) string {
    out, err := exec.Command("iwgetid", name, "--raw").Output()
    if err != nil {
        log.Println("Cannot obtain SSID:", err)
        return ""
    }
    return strings.TrimSuffix(string(out), "\n")
}

func defaultWlan() string {
    out, err := raspi_config_nonint("list_wlan_interfaces")
    if err != nil {
        log.Println("Cannot obtain default wlan:", err)
        return ""
    }

    // Imitate raspi-config taking the first line.
    return strings.SplitN(out, "\n", 2)[0]
    // In the case of no wlan, output will be "\n".
    // SplitN("\n", "\n", 1) return ["\n"], but I want an empty string.
    // Thus, '2' above.
}

func gatherInterfaces() NetworkInterfaceMap {
    wlan00 := defaultWlan()
    isDefaultWlan := func (n string) bool {
        return n == wlan00
    }

    ifmap := make(NetworkInterfaceMap)
    ifaces, err := net.Interfaces()
    if err != nil {
        log.Println("Cannot obtain network interfaces:", err)
        return ifmap
    }

    for _,i := range ifaces {
        if i.Name == "lo" { continue }

        ps := NewStringSet()
        addrs, err := i.Addrs()
        if err != nil {
            ifmap[i.Name] = NetworkInterface{
                                    Name: i.Name,
                                    IPs: ps,
                                    WiFi: isDefaultWlan(i.Name)}
            log.Printf("Cannot obtain addresses for %v: %v", i.Name, err)
            continue
        }

        for _,a := range addrs {
            switch b := a.(type) {
            case *net.IPNet:
                ps.Add(b.IP.String())
            case *net.IPAddr:
                ps.Add(b.IP.String())
            }
        }

        if strings.HasPrefix(i.Name, "wlan") && ps.Size() > 0 {
            ssid := findSSID(i.Name)
            ifmap[i.Name] = NetworkInterface{
                                    Name: i.Name,
                                    IPs: ps,
                                    SSID: ssid,
                                    WiFi: isDefaultWlan(i.Name)}
        } else {
            ifmap[i.Name] = NetworkInterface{
                                    Name: i.Name,
                                    IPs: ps,
                                    WiFi: isDefaultWlan(i.Name)}
        }
    }
    return ifmap
}

func processExists(name string) bool {
    err := exec.Command("pgrep", name).Run()
	  return err == nil
}

func gatherServices() ServiceMap {
    return ServiceMap{
        "SSH": Service{"SSH", processExists("sshd")},
        "VNC": Service{"VNC", processExists("vncserver")},
    }
}

func inspectSystem() *SystemStates {
    return &SystemStates{gatherInterfaces(), gatherServices()}
}

type SystemReport struct {
    Full       bool
    Interfaces []NetworkInterface
    Services   []Service
}

func produceFullReport(s *SystemStates) *SystemReport {
    return &SystemReport{true, s.Interfaces.Values(), s.Services.Values()}
}

func produceReport(new *SystemStates, old *SystemStates) *SystemReport {
    if !new.Interfaces.Keys().Equal(old.Interfaces.Keys()) ||
            !new.Services.Keys().Equal(old.Services.Keys()) {
        return produceFullReport(new)
    }

    var ifaces []NetworkInterface
    for name, f := range new.Interfaces {
        if !f.Equal(old.Interfaces[name]) {
            ifaces = append(ifaces, f)
        }
    }

    var servs []Service
    for name, s := range new.Services {
        if s != old.Services[name] {
            servs = append(servs, s)
        }
    }

    if ifaces == nil && servs == nil {
        return nil
    } else {
        return &SystemReport{false, ifaces, servs}
    }
}

const (
    MonitorStart = 1 << iota
    MonitorBurst
    MonitorStop
)

func MonitorSystemStates(in <-chan int, out chan<- *SystemReport, notify chan<- int, id int) {
    defer RecoverDo(
        func(x interface{}) {
            notify <- id
            log.Println("Monitor terminates due to:", x)
        },
        func() {
            log.Println("Monitor terminates normally")
        },
    )

    var current *SystemStates

    // Wait for first control code ...
    ctrl, ok := <-in
    if !ok {
        panic("First control code never arrives")
    }

    // ... which must be MonitorStart
    switch ctrl {
    case MonitorStart:
        s := inspectSystem()
        r := produceFullReport(s)

        current = s
        out <- r
    default:
        panic(fmt.Sprintf("Invalid first control code: %v", ctrl))
    }

    // Regular report interval = 3 sec
    regularTicker := time.NewTicker(3 * time.Second)
    defer regularTicker.Stop()
    active := true

    // Report more frequently on receiving MonitorBurst
    burstTicker := time.NewTicker(1200 * time.Millisecond)
    defer burstTicker.Stop()
    bursts := 0

    for {
        select {
        case ctrl, ok = <-in:
            if !ok { return }

            switch ctrl {
            case MonitorStart:
                active = true
                s := inspectSystem()
                r := produceFullReport(s)

                current = s
                out <- r

            case MonitorBurst:
                bursts = 9

            case MonitorStop:
                bursts = 0
                active = false

            default:
                panic(fmt.Sprintf("Invalid monitor control code: %v", ctrl))
            }
        case <-regularTicker.C:
            if active {
                s := inspectSystem()
                r := produceReport(s, current)

                current = s
                out <- r
            }

        case <-burstTicker.C:
            if active && bursts > 0 {
                bursts--
                s := inspectSystem()
                r := produceReport(s, current)

                current = s
                out <- r
            }
        }
    }
}
