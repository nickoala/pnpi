package main

import (
    "github.com/google/gousb"
    "fmt"
    "log"
    "time"
    "strings"
)

type N int

func (n N) in(ns ...int) bool {
    for _,i := range ns {
        if int(n) == i { return true }
    }
    return false
}

type DeviceIdentity struct {
    Bus int
    Address int
    Vendor gousb.ID
    Product gousb.ID
}

func ReadDeviceIdentity(d *gousb.DeviceDesc) DeviceIdentity {
    return DeviceIdentity{d.Bus, d.Address, d.Vendor, d.Product}
}

func (i DeviceIdentity) Nil() bool {
    return i.Bus == 0 && i.Address == 0 && i.Vendor == 0 && i.Product == 0
}

func (i DeviceIdentity) Match(d *gousb.DeviceDesc) bool {
    return i.Bus == d.Bus &&
           i.Address == d.Address &&
           i.Vendor == d.Vendor &&
           i.Product == d.Product
}

func (i DeviceIdentity) IsAccessoryMode() bool {
    return i.Vendor == 0x18D1 && N(i.Product).in(0x2D00, 0x2D01)
}

type DeviceHistory int

const (
    historyNoAction = iota
    historySwitchRequested
    historySwitchFailed
    historyOpenFailed
)

type DeviceMap map[DeviceIdentity]DeviceHistory

func mapDevices() DeviceMap {
    ctx := gousb.NewContext()
    defer ctx.Close()

    m := make(DeviceMap)
    ctx.OpenDevices(func(d *gousb.DeviceDesc) bool {
        m[ReadDeviceIdentity(d)] = historyNoAction
        return false
    })

    return m
}

func propagateDeviceHistory(i DeviceIdentity, h DeviceHistory) DeviceHistory {
    if i.IsAccessoryMode() {
        if h == historyOpenFailed {
            return historyOpenFailed
        } else {
            return historyNoAction
        }
    } else {
        if h == historySwitchRequested {
            log.Printf("Not yet switched, treat as failed: %v", i)
            return historySwitchFailed
        } else {
            return h
        }
    }
}

func updateDeviceMap(new DeviceMap, old DeviceMap) (DeviceMap, DeviceIdentity, DeviceIdentity) {
    var identityOfAccessoryMode, identityToSwitch DeviceIdentity
    m := make(DeviceMap)

    for identity, blank := range new {
        history, ok := old[identity]
        if ok {
            history = propagateDeviceHistory(identity, history)
        } else {
            history = blank
        }

        m[identity] = history

        if identity.IsAccessoryMode() && history == historyNoAction {
            identityOfAccessoryMode = identity
        } else if !identity.IsAccessoryMode() && history == historyNoAction {
            identityToSwitch = identity
        }
    }
    return m, identityOfAccessoryMode, identityToSwitch
}

func findConfig(d *gousb.DeviceDesc) (*gousb.ConfigDesc, error) {
    if len(d.Configs) <= 0 {
        return nil, fmt.Errorf("No config descriptor found")
    }

    var found = false
    var lowest int
    var cfg gousb.ConfigDesc

    for n,c := range d.Configs {
        if !found || n < lowest {
            found, lowest = true, n
            cfg = c
        }
    }
    return &cfg, nil
}

func findInterface(c *gousb.ConfigDesc) (*gousb.InterfaceSetting, error) {
    if len(c.Interfaces) <= 0 {
        return nil, fmt.Errorf("No interface descriptor found")
    }

    infDesc := &(c.Interfaces[0])

    if len(infDesc.AltSettings) <= 0 {
        return nil, fmt.Errorf("No interface alternate setting found")
    }

    return &(infDesc.AltSettings[0]), nil
}

func findEndpoints(s *gousb.InterfaceSetting) (*gousb.EndpointDesc, *gousb.EndpointDesc, error) {
    var inFound, outFound = false, false
    var in, out gousb.EndpointDesc

    for _,desc := range s.Endpoints {
        switch desc.Direction {
        case gousb.EndpointDirectionIn:
            inFound, in = true, desc
        case gousb.EndpointDirectionOut:
            outFound, out = true, desc
        }
    }

    switch {
    case !inFound && !outFound:
        return nil, nil, fmt.Errorf("No endpoint found")
    case !inFound:
        return nil, nil, fmt.Errorf("No IN-endpoint found")
    case !outFound:
        return nil, nil, fmt.Errorf("No OUT-endpoint found")
    default:
        return &in, &out, nil
    }
}

type Errors []error

func (es Errors) Error() string {
    var ss []string
    for _,e := range es {
        ss = append(ss, e.Error())
    }
    return strings.Join(ss, "\n")
}

type AccessoryModeStack struct {
    Context *gousb.Context
    Device *gousb.Device
    Config *gousb.Config
    Interface *gousb.Interface
    InEndpoint *gousb.InEndpoint
    OutEndpoint *gousb.OutEndpoint
    ReadStream *gousb.ReadStream
}

func (s *AccessoryModeStack) Close() error {
    var e error
    var errs Errors

    if s.ReadStream != nil {
        e = s.ReadStream.Close()
        if e != nil { errs = append(errs, e) }
    }

    if s.Interface != nil {
        s.Interface.Close()
    }

    if s.Config != nil {
        e = s.Config.Close()
        if e != nil { errs = append(errs, e) }
    }

    if s.Device != nil {
        e = s.Device.Close()
        if e != nil { errs = append(errs, e) }
    }

    if s.Context != nil {
        e = s.Context.Close()
        if e != nil { errs = append(errs, e) }
    }

    if len(errs) > 0 {
        return errs
    }
    return nil
}

func openStack(i DeviceIdentity) (*AccessoryModeStack, error) {
    var err error
    var stack AccessoryModeStack
    defer func() {
        if err != nil {
            log.Printf("Cannot open stack: %v, %v", i, err)
            stack.Close()
        }
    }()

    stack.Context = gousb.NewContext()

    var devDesc *gousb.DeviceDesc
    ds, err := stack.Context.OpenDevices(func(d *gousb.DeviceDesc) bool {
        if i.Match(d) {
            devDesc = d  // remember for later inspection
            return true
        }
        return false
    })

    if err != nil {
        for _,d := range ds { d.Close() }
        return nil, err
    }

    if len(ds) < 1 {
        for _,d := range ds { d.Close() }
        err = fmt.Errorf("No device found: %v", i)
        return nil, err
    }

    if len(ds) > 1 {
        for _,d := range ds { d.Close() }
        err = fmt.Errorf("More than one device found: %v", i)
        return nil, err
    }

    stack.Device = ds[0]

    cfgDesc, err := findConfig(devDesc)
    if err != nil {
        return nil, err
    }

    stack.Config, err = stack.Device.Config(cfgDesc.Number)
    if err != nil {
        return nil, err
    }

    infSetting, err := findInterface(cfgDesc)
    if err != nil {
        return nil, err
    }

    stack.Interface, err = stack.Config.Interface(infSetting.Number, infSetting.Alternate)
    if err != nil {
        return nil, err
    }

    epinDesc, epoutDesc, err := findEndpoints(infSetting)
    if err != nil {
        return nil, err
    }

    stack.InEndpoint, err = stack.Interface.InEndpoint(epinDesc.Number)
    if err != nil {
        return nil, err
    }

    stack.OutEndpoint, err = stack.Interface.OutEndpoint(epoutDesc.Number)
    if err != nil {
        return nil, err
    }

    stack.ReadStream, err = stack.InEndpoint.NewStream(epinDesc.MaxPacketSize, 2)
    if err != nil {
        return nil, err
    }

    return &stack, nil
}

const (
    DataDirectionIn = 0x80
    DataDirectionOut = 0x00
)

func controlRequestIn(d *gousb.Device, request uint8, val, idx uint16, data []byte) int {
    x,err := d.Control(DataDirectionIn | gousb.RequestTypeVendor, request, val, idx, data)
    if err != nil {
        panic(err)
    }
    return x
}

func controlRequestOut(d *gousb.Device, request uint8, val, idx uint16, data []byte) int {
    x,err := d.Control(DataDirectionOut | gousb.RequestTypeVendor, request, val, idx, data)
    if err != nil {
        panic(err)
    }
    return x
}

func switchToAccessoryMode(d *gousb.Device) (err error) {
    defer func() {
        e := recover()
        if e != nil {
            err = e.(error)
        } else {
            err = nil
        }
    }()

    const (
        manufacturer = "Nick Lee of Hong Kong"
        model = "Plug n Pi Server"
        description = "The Raspberry side of Plug n Pi"
        protocolVersion = "1"
        uri = "https://github.com/nickoala/pnpi"
        serialNumber = "0123456789"
    )

    version := controlRequestIn(d, 51, 0, 0, []byte{0x00,0x00})
    if !N(version).in(1, 2) {
        panic(fmt.Errorf("Invalid AOA version number: %v", version))
    }

    controlRequestOut(d, 52, 0, 0, []byte(manufacturer + "\x00"))
    controlRequestOut(d, 52, 0, 1, []byte(model + "\x00"))
    controlRequestOut(d, 52, 0, 2, []byte(description + "\x00"))
    controlRequestOut(d, 52, 0, 3, []byte(protocolVersion + "\x00"))
    controlRequestOut(d, 52, 0, 4, []byte(uri + "\x00"))
    controlRequestOut(d, 52, 0, 5, []byte(serialNumber + "\x00"))
    controlRequestOut(d, 53, 0, 0, nil)
    return nil
}

func requestSwitch(i DeviceIdentity) error {
    ctx := gousb.NewContext()
    defer ctx.Close()

    ds, err := ctx.OpenDevices(i.Match)
    for _,d := range ds { defer d.Close() }
    if err != nil {
        return err
    }

    if len(ds) < 1 {
        return fmt.Errorf("No device found: %v", i)
    }

    if len(ds) > 1 {
        return fmt.Errorf("More than one device found: %v", i)
    }

    return switchToAccessoryMode(ds[0])
}

var currentDeviceMap = make(DeviceMap)

func OpenAccessoryModeStack() *AccessoryModeStack {
    for {
        m, identityOfAccessoryMode, identityToSwitch :=
                        updateDeviceMap(mapDevices(), currentDeviceMap)
        currentDeviceMap = m

        if !identityOfAccessoryMode.Nil() {
            stack, err := openStack(identityOfAccessoryMode)
            if err == nil {
                log.Printf("Accessory mode opened: %v", identityOfAccessoryMode)
                return stack
            }
            log.Printf("Cannot open accessory mode: %v, %v", identityOfAccessoryMode, err)
            currentDeviceMap[identityOfAccessoryMode] = historyOpenFailed
        }

        if !identityToSwitch.Nil() {
            log.Printf("Requesting switch: %v", identityToSwitch)
            err := requestSwitch(identityToSwitch)
            if err != nil {
                log.Printf("Cannot switch to accessory mode: %v, %v", identityToSwitch, err)
                currentDeviceMap[identityToSwitch] = historySwitchFailed
            } else {
                log.Printf("Switch to accessory mode requested: %v", identityToSwitch)
                currentDeviceMap[identityToSwitch] = historySwitchRequested

                log.Println("Wait 1 second for it to come on bus again")
                time.Sleep(1 * time.Second)
            }
        } else {
            // Nothing to switch, wait a bit before checking again.
            time.Sleep(2 * time.Second)
        }
    }
}
