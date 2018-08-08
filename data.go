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

type SystemStates struct {
    Type string                   `json:"type"`
    Interfaces []NetworkInterface `json:"interfaces"`
    Services []Service            `json:"services"`
    WifiCountryCode string        `json:"wifi_country_code"`
}

func NewSystemStates(nis []NetworkInterface, ss []Service, wc string) *SystemStates {
    return &SystemStates { "states", nis, ss, wc }
}

type SystemStatesChange struct {
    Type string                   `json:"type"`
    Interfaces []NetworkInterface `json:"interfaces,omitempty"`
    Services []Service            `json:"services,omitempty"`
    WifiCountryCode string        `json:"wifi_country_code"`
    // WifiCountryCode: Don't omitempty, may want to pass empty string.
    // Because it's always present, the field should always contain the latest country code.
}

func NewSystemStatesChange(nis []NetworkInterface, ss []Service, wc string) *SystemStatesChange {
    return &SystemStatesChange { "change", nis, ss, wc }
}

type Hotspot struct {
    SSID string  `json:"ssid"`
    Open bool    `json:"open"`
    Signal int   `json:"signal"`
}

type ScanResult struct {
    Type string        `json:"type"`
    Hotspots []Hotspot `json:"hotspots"`
}

func NewScanResult(hs []Hotspot) *ScanResult {
    return &ScanResult { "scan", hs }
}

type Country struct {
    Code string `json:"code"`
    Name string `json:"name"`
}

type SystemChoices struct {
    Type string         `json:"type"`
    Countries []Country `json:"countries"`
}

func NewSystemChoices(cs []Country) *SystemChoices {
    return &SystemChoices { "choices", cs }
}

type Command struct {
    Action string `json:"action"`
    Args []string `json:"args,omitempty"`
}

func (c *Command) String() string {
    return fmt.Sprintf("{%v %v}", c.Action, c.Args)
}
