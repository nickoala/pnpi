# Plug n Pi Protocol

This is as much for my own reference as for everyone else.

On connection, client sends a `monitor start` command, to which server responds
with:

- initially, a `choices` object conveying Raspberry Pi's supported WiFi country
  codes

- followed by a `states` object detailing Raspberry Pi's network interfaces and
  service status

- subsequently, if any changes occur to relevant network interfaces or services,
  a `change` object is sent

- if no change occurs, an empty object `{}` is sent every few seconds

When user leaves the app's MainActivity, a `monitor stop` command is sent to
pause server monitoring.

All "commands" and "objects" are JSON-serialized.

```
client                                                       server
       ------ {"action":"monitor", "args":["start"]} ------>

       <------------- {"type":"choices", ...} --------------
       <------------- {"type":"states", ...} ---------------
       <------------- {"type":"change", ...} ---------------
       <----------------------- {} -------------------------

       ------ {"action":"monitor", "args":["stop"]} ------->
```

When user enters the app's HotspotActivity (meaning he wants to connect WiFi), a
`scan start` command is sent, to which server responds with:

- a `scan` object listing available hotspots, every few seconds

When user leaves the app's HotspotActivity, a `scan stop` command is sent to
pause server scanning.

```
client                                                       server
       -------- {"action":"scan", "args":["start"]} ------->

       <-------------- {"type":"scan", ...} ----------------

       -------- {"action":"scan", "args":["stop"]} -------->
```

Additional commands in response to user actions:

```
client                                                           server
       --- {"action":"country", "args":[ country code ]} ------>
       --- {"action":"connect", "args":[ SSID, passphrase ]} -->
       --- {"action":"disconnect", "args":[ SSID ]} ----------->
       --- {"action":"start", "args":[ service name ]} -------->
       --- {"action":"stop", "args":[ service name ]} --------->
       --- {"action":"halt", "args":[]} ----------------------->
       --- {"action":"reboot", "args":[]} --------------------->
```
