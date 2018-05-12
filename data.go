package main

import  "fmt"

type NetworkInterface struct {
    Name string    `json:"name"`
    IPs *StringSet `json:"ip,omitempty"`
    SSID string    `json:"ssid,omitempty"`
    WiFi bool      `json:"wifi"`
}

func (i NetworkInterface) Equal(j NetworkInterface) bool {
    return i.Name == j.Name &&
           i.IPs.Equal(j.IPs) &&
           i.SSID == j.SSID &&
           i.WiFi == j.WiFi
}

type Service struct {
    Name string  `json:"name"`
    Running bool `json:"running"`
}

type Hotspot struct {
    SSID string  `json:"ssid"`
    Open bool    `json:"open"`
    Signal int   `json:"signal"`
}

type Report struct {
    Type string                   `json:"type"`
    Interfaces []NetworkInterface `json:"interfaces,omitempty"`
    Services   []Service          `json:"services,omitempty"`
    Hotspots   []Hotspot          `json:"hotspots,omitempty"`
}

type Command struct {
    Action string `json:"action"`
    Args []string `json:"args,omitempty"`
}

func (c *Command) String() string {
    return fmt.Sprintf("{%v %v}", c.Action, c.Args)
}
