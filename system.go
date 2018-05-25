package main

import (
    "strings"
    "os/exec"
    "path/filepath"
)

var ScriptDirectory = ""

func raspi_config(a ...string) (string, error) {
    b, err := exec.Command(filepath.Join(ScriptDirectory, "raspi-config"), a...).Output()
    return string(b), err
}

func service_functions(name string) (string, string) {
    switch name {
    case "SSH": return "do_ssh", "get_ssh"
    case "VNC": return "do_vnc", "get_vnc"
    default: panic("Invalid service name: " + name)
    }
}

func HaltSystem() error {
    return exec.Command("halt", "-h").Run()
}

func RebootSystem() error {
    return exec.Command("reboot").Run()
}

func StartService(name string) error {
    doer, _ := service_functions(name)
    _, err := raspi_config(doer, "0")
    return err
}

func StopService(name string) error {
    doer, _ := service_functions(name)
    _, err := raspi_config(doer, "1")
    return err
}

func WifiConnect(ssid string, passphrase string) error {
    _, err := raspi_config("do_wifi_ssid_passphrase", ssid, passphrase)
    return err
}

func WifiDisconnect(ssid string) error {
    _, err := raspi_config("do_wifi_ssid_disconnect", ssid)
    return err
}

func ServiceIsRunning(name string) (bool, error) {
    _, getter := service_functions(name)
    status, err := raspi_config(getter)
    if err != nil {
        return false, err
    }
    return (strings.TrimSpace(status) == "0"), err
}

func DefaultWlanInterface() (string, error) {
    out, err := raspi_config("list_wlan_interfaces")
    if err != nil {
        return "", err
    }

    // Imitate raspi-config taking the first line.
    return strings.SplitN(out, "\n", 2)[0], err
    // In the case of no wlan, output will be "\n".
    // SplitN("\n", "\n", 1) return ["\n"], but I want an empty string.
    // Thus, '2' above.
}

func ReportSsid(name string) (string, error) {
    out, err := exec.Command("iwgetid", name, "--raw").Output()
    if err != nil {
        return "", err
    }
    return strings.TrimSuffix(string(out), "\n"), err
}
