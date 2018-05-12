package main

import "os/exec"

func raspi_config_nonint(a ...string) (string, error) {
    z := append([]string{"nonint"}, a...)
    b, err := exec.Command("raspi-config", z...).Output()
    return string(b), err
}

func service_fn(name string) string {
    switch name {
    case "SSH": return "do_ssh"
    case "VNC": return "do_vnc"
    default: panic("Invalid service name: " + name)
    }
}
