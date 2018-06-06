# Plug n Pi

1. USB-connect a Raspberry Pi and a Mobile Phone

2. The Phone's app allows you to:
   - connect Raspberry Pi to a WiFi hotspot
   - look up Raspberry Pi's IP addresses
   - enable Raspberry Pi's SSH and VNC server

   In short, everything you need to gain network access to your Raspberry Pi,
   except the terminal emulator and VNC client.

3. Use your favorite terminal emulator (PuTTY?) or VNC client (RealVNC?)
   to enter the Pi

Plug n Pi's software consists of two parts:

1. On the Pi side, there is the USB server. **This page concerns the server (and
   general info)**. The server has been tested on **Raspbian Stretch** on:

   - Model 3 B+
   - Model 3 B
   - Zero W

2. On the Phone side, there is the USB client. I've only written the [app for
   Android](https://github.com/nickoala/pnpi-android). No iPhone support in the
   foreseeable future. The Android app has been tested on the following devices:

|          Model         | Android version | API level |
|:----------------------:|:---------------:|:---------:|
| Samsung Galaxy Express |           4.1.2 |        16 |
| ASUS Fondpad ?         |           4.1.2 |        16 |
| Samsung Galaxy S4      |           5.0.1 |        21 |
| Samsung Tab A          |           7.1.1 |        25 |

## What problems am I trying to solve?

Imagine a room with 10 people who have their own Raspberry Pi. The room is not
dedicated to the usage of Single Board Computers, so there is no monitor,
keyboard, and mouse available. Everyone brings their own laptops. The only way
to work on Raspberry Pi is remote login over network. (Let's forget about
console cables. Very few people use it.)

The room has a WiFi hotspot, but the hotspot/router belongs to the company, not
under your control. (But, of course, you know the SSID and passphrase.
Otherwise, there would be no hope of using it.)

#### Problem #1: How to connect Raspberry Pi to the hotspot?

Solution: Before inserting the SD card into the Pi, insert a file
`wpa_supplicant.conf` with the correct credentials into the SD card's boot
partition, presumably after looking up what the file should look like on the
web. Inconvenient for the experienced, arduous for the newly initiated.

#### Problem #2: What is my Pi's IP address?

(Non-)Solution #1: Look up the DHCP clients on the hotspot/router. This can be
done in a home, but rarely do you get this kind of access in an organizational
setting, as in the current scenario.

(Non-)Solution #2: Make use of multicast DNS and type `raspberrypi.local`
instead. This works on a one-man island. In our 10-man room, people may have
freshly imaged their SD cards, all of which have identical hostnames! (You can
change hostname by editing some files on the SD card pre-boot. Again,
inconvenient for the experienced, arduous for the newly initiated.)

Furthermore, some network blocks multicast DNS traffic, so `whatever.local`
simply wouldn't work.

(Non-)Solution #3: Network scan. Again, this works better on a one-man island.
Here, the scanner may not be able to display the hostname of each Pi (even if
they are different). All people in the room are left wondering which IP belongs
to which Pi belongs to whom!

In real life, I've seen people bringing their own personal hotspots. One Hotspot
per Pi (or two). I think it's ridiculous.

Solution #4: Equip every Pi with an LCD touchscreen. Somehow drag the mouse
pointer to the network icon, for the IP address to pop up. An LCD for an IP
address? Yes, it's an overkill.

**In short, there are ways to deal with the problems. None is satisfactory.
I want a better solution.**

## How does it work?

Normally, when a phone is USB-plugged into a computer (Pi included), the default
communication is on MTP (Media Transfer Protocol), for the two to transfer
files. Android has a way to escape from MTP and allows a computer to talk to the
phone in a more generic manner.

The computer implements [Android Open Accessory
Protocol](https://source.android.com/devices/accessories/aoa) which constitutes
sending some specific control requests to the phone over USB. In response, the
phone switch to so-called [Accessory
Mode](https://developer.android.com/guide/topics/connectivity/usb/accessory),
and the computer becomes (to the phone) a USB accessory. They can freely
exchange bytes afterwards. Plug n Pi implements a custom protocol on top of this
data connection.

At all times, the computer serves as the USB host and provides power to the USB
bus. This suits Raspberry Pi because the "big" ones (3B, 3B+) can only act as
USB hosts, not USB devices.

## Build

Raspbian package is not yet available. For now, you have to build the server
yourself.

Install the Go language compiler and libusb:
```
sudo apt-get install golang libusb-1.0-0 libusb-1.0-0-dev
```

Obtain `gousb`, Go's USB package. Normally, we do that with `go get`, but
`gousb` has introduced some breaking changes recently. I need an older version.
The following gets an older version while preserving Go's directory conventions:
```
mkdir ~/pnpi
cd ~/pnpi
git clone https://github.com/google/gousb src/github.com/google/gousb
cd src/github.com/google/gousb
git checkout d036636
```

Obtain Plug n Pi Server's source code:
```
cd ~/pnpi
git clone https://github.com/nickoala/pnpi src/pnpi
```

Build it:
```
export GOPATH=`pwd`
go build pnpi
```

You should have an executable file named `pnpi` in the working directory.

## Run

Plug n Pi Server requires a shell script `raspi-config` (customized from the
[official configuration tool](https://github.com/RPi-Distro/raspi-config)) to
perform some system operations. It has to be placed alongside the `pnpi`
executable before they can run properly:

```
cp src/pnpi/raspi-config .
chmod +x raspi-config
sudo ./pnpi
```

Now, plug in the Phone (with [the app](https://github.com/nickoala/pnpi-android)
installed) to see how it works.

## Auto-start

I use systemd's path-based activation (thanks to [Mark
Stosberg](https://superuser.com/a/1322879/762013) for the suggestion): once the
USB bus is ready, start Plug n Pi Server. Two files have to be created under the
directory `/etc/systemd/system/`.

First, the service unit, named `pnpi.service`:
```
[Unit]
Description=Plug n Pi Server

[Service]
ExecStart=/home/pi/pnpi/pnpi
User=root
```

Second, the path unit, named `pnpi.path`:
```
[Unit]
Description=Monitor USB bus ready

[Path]
DirectoryNotEmpty=/dev/bus/usb/001

[Install]
WantedBy=multi-user.target
```

The path unit will activate the service unit when the directory
`/dev/bus/usb/001` becomes non-empty, i.e. when USB bus is ready.

Enable the path unit:
```
$ sudo systemctl enable pnpi.path
```

## Q&A

#### I've plugged in the Pi, but the Phone/App never reacts. I'm sure Pi is powered on.

Check if Phone is charging ...

If it is not:

- **The USB cable connecting Pi and Phone may not be fully plugged in.**

If it is:

- A USB cable usually includes two pairs of wires: one for passing power, one
  for passing data. Some cables only have the power pair. For Plug n Pi to work,
  **the USB cable connecting Pi and Phone must have data wires.**

- If Pi is under-voltage, it may not be able to enumerate USB devices properly
  (although it can still charge the phone somewhat). **Try using a better power
  adapter and a better power cable to power the Pi.**

#### App keeps disconnecting from Pi, for no apparent reason.

A sign that Pi may be under-voltage. **Try using a better power adapter and a
better power cable to power the Pi.**

#### App keeps telling me "Plug n Pi Server is not enabled on Pi". But I'm sure it's enabled.

- **Try removing all USB attachments from Pi, then re-plug Phone to Pi.** When
  more than one USB devices are plugged in, Pi does not know which one is the
  Phone. It queries each device in turn. Most USB devices give a definite
  answer, but some USB dongles confuse the Pi.

- For some reason, Phone may be unable to switch to Android accessory mode.
  **Try re-starting Phone.**

#### App is showing Pi's IP address(es) and SSH is turned on, but I still can't SSH in!

Remember, **your computer has to be on the same LAN with Pi** for the IP
address(es) to be useful.

#### Do I lose the ability to transfer files between Phone and Pi?

Yes. To transfer files, Phone and Pi have to be on MTP. Plug n Pi forces them to
leave MTP and talk on Android Open Accessory Protocol. To be able to transfer
files again, Plug n Pi Server has to be disabled.
