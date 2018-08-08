package main

import (
    "strings"
    "os"
    "os/exec"
    "path/filepath"
)

var scriptDirectory = ""

func SetScriptDirectory(d string) {
    scriptDirectory = d
}

func CheckScript() {
    // Ensure raspi-config present
    path := filepath.Join(scriptDirectory, "raspi-config")
    info, err := os.Stat(path)
    if err != nil {
        LogFatalf("%s not found: %v", path, err)
    }

    // Check executable bits
    mode := info.Mode()
    if (mode & 0x49 != 0x49) {  // 0x49 == 001001001
        LogFatalf("%s must be executable, e.g. rwxr-xr-x", path)
    }
}

func raspi_config(a ...string) (string, error) {
    b, err := exec.Command(filepath.Join(scriptDirectory, "raspi-config"), a...).Output()
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
    return strings.SplitN(strings.TrimSpace(out), "\n", 2)[0], err
}

func ReportSsid(name string) (string, error) {
    out, err := exec.Command("iwgetid", name, "--raw").Output()
    if err != nil {
        return "", err
    }
    return strings.TrimSuffix(string(out), "\n"), err
}

func WifiCountryCode() (string, error) {
    out, err := raspi_config("get_wifi_country")
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(out), err
}

func AvailableWifiCountries() ([]Country, error) {
    out, err := raspi_config("list_wifi_countries")
    if err != nil {
        return make([]Country, 0), err
    }

    lines := strings.Split(strings.TrimSpace(out), "\n")
    countries := make([]Country, len(lines))

    for i,n := range lines {
        c := strings.SplitN(n, ",", 2)  // split into country code, name
        countries[i] = Country{ c[0], c[1] }
    }
    return countries, err
}

func SetWifiCountry(code string) error {
    _, err := raspi_config("do_wifi_country", code)
    return err
}
