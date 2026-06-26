//go:build linux

// Package input provides virtual keyboard and mouse device management via
// Linux's /dev/uinput subsystem. It enables Remote Studio to inject
// keyboard and mouse events into the local display server without requiring
// a physical input device — essentially a software KVM switch.
package input

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Linux uinput constants — these mirror the kernel headers.
// ---------------------------------------------------------------------------

const (
	uinputPath = "/dev/uinput"

	// ioctl directions
	iocWrite = 1
	iocRead  = 2

	// uinput ioctl commands
	uiSetEvbit  = 0x40045564 // UI_SET_EVBIT
	uiSetKeybit = 0x40045565 // UI_SET_KEYBIT
	uiSetRelbit = 0x40045566 // UI_SET_RELBIT
	uiSetAbsbit = 0x40045567 // UI_SET_ABSBIT

	uiDevCreate  = 0x5501 // UI_DEV_CREATE
	uiDevDestroy = 0x5502 // UI_DEV_DESTROY

	// Maximum length of the device name in uinput_user_dev.
	uinputMaxNameSize = 80

	// Input event types (EV_*)
	evSyn = 0x00
	evKey = 0x01
	evRel = 0x02
	evAbs = 0x03

	// Relative axes (REL_*)
	relX = 0x00
	relY = 0x01

	// Synchronization events
	synReport = 0x00

	// Key event values
	keyRelease = 0
	keyPress   = 1

	// BUS_USB for the fake device
	busUSB = 0x03
)

// ---------------------------------------------------------------------------
// uinput_user_dev — matches the kernel struct layout.
// ---------------------------------------------------------------------------

// uinputUserDev mirrors struct uinput_user_dev from <linux/uinput.h>.
// The absmax/absmin/absfuzz/absflat arrays have 64 entries (ABS_CNT).
type uinputUserDev struct {
	Name [uinputMaxNameSize]byte
	ID   inputID
	// We must include the abs arrays to match the kernel struct size
	// even though we may not use them for every device.
	EffectsMax uint32
	Absmax     [64]int32
	Absmin     [64]int32
	Absfuzz    [64]int32
	Absflat    [64]int32
}

// inputID mirrors struct input_id.
type inputID struct {
	Bustype uint16
	Vendor  uint16
	Product uint16
	Version uint16
}

// inputEvent mirrors struct input_event for 64-bit architectures.
type inputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// ---------------------------------------------------------------------------
// DeviceType describes what kind of virtual device to create.
// ---------------------------------------------------------------------------

// DeviceType indicates whether a VirtualDevice is a keyboard or mouse.
type DeviceType int

const (
	DeviceKeyboard DeviceType = iota
	DeviceMouse
)

// VirtualDevice wraps a uinput file descriptor representing either a
// virtual keyboard or a virtual mouse.
type VirtualDevice struct {
	file       *os.File
	name       string
	deviceType DeviceType
}

// ---------------------------------------------------------------------------
// Public constructors
// ---------------------------------------------------------------------------

// CreateVirtualKeyboard opens /dev/uinput and registers a virtual keyboard
// device with the given human-readable name.
func CreateVirtualKeyboard(name string) (*VirtualDevice, error) {
	f, err := os.OpenFile(uinputPath, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w (are you root?)", uinputPath, err)
	}

	// Enable EV_KEY
	if err := ioctl(f, uiSetEvbit, uintptr(evKey)); err != nil {
		f.Close()
		return nil, fmt.Errorf("UI_SET_EVBIT EV_KEY: %w", err)
	}

	// Register all standard keys (KEY_RESERVED=0 through KEY_MAX=0x2ff).
	for key := 0; key <= 0x2ff; key++ {
		if err := ioctl(f, uiSetKeybit, uintptr(key)); err != nil {
			f.Close()
			return nil, fmt.Errorf("UI_SET_KEYBIT %d: %w", key, err)
		}
	}

	// Enable EV_SYN (always needed)
	if err := ioctl(f, uiSetEvbit, uintptr(evSyn)); err != nil {
		f.Close()
		return nil, fmt.Errorf("UI_SET_EVBIT EV_SYN: %w", err)
	}

	// Write the device description
	if err := writeUserDev(f, name); err != nil {
		f.Close()
		return nil, err
	}

	// Create the device
	if err := ioctl(f, uiDevCreate, 0); err != nil {
		f.Close()
		return nil, fmt.Errorf("UI_DEV_CREATE: %w", err)
	}

	// Small delay so udev can set up the device node.
	time.Sleep(200 * time.Millisecond)

	return &VirtualDevice{
		file:       f,
		name:       name,
		deviceType: DeviceKeyboard,
	}, nil
}

// CreateVirtualMouse opens /dev/uinput and registers a virtual relative
// mouse device with the given human-readable name.
func CreateVirtualMouse(name string) (*VirtualDevice, error) {
	f, err := os.OpenFile(uinputPath, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w (are you root?)", uinputPath, err)
	}

	// Enable EV_KEY for mouse buttons
	if err := ioctl(f, uiSetEvbit, uintptr(evKey)); err != nil {
		f.Close()
		return nil, fmt.Errorf("UI_SET_EVBIT EV_KEY: %w", err)
	}

	// Register mouse buttons: BTN_LEFT(0x110), BTN_RIGHT(0x111), BTN_MIDDLE(0x112)
	for btn := 0x110; btn <= 0x112; btn++ {
		if err := ioctl(f, uiSetKeybit, uintptr(btn)); err != nil {
			f.Close()
			return nil, fmt.Errorf("UI_SET_KEYBIT btn 0x%x: %w", btn, err)
		}
	}

	// Enable EV_REL for relative axis movement
	if err := ioctl(f, uiSetEvbit, uintptr(evRel)); err != nil {
		f.Close()
		return nil, fmt.Errorf("UI_SET_EVBIT EV_REL: %w", err)
	}

	// Register REL_X and REL_Y
	if err := ioctl(f, uiSetRelbit, uintptr(relX)); err != nil {
		f.Close()
		return nil, fmt.Errorf("UI_SET_RELBIT REL_X: %w", err)
	}
	if err := ioctl(f, uiSetRelbit, uintptr(relY)); err != nil {
		f.Close()
		return nil, fmt.Errorf("UI_SET_RELBIT REL_Y: %w", err)
	}

	// Enable EV_SYN
	if err := ioctl(f, uiSetEvbit, uintptr(evSyn)); err != nil {
		f.Close()
		return nil, fmt.Errorf("UI_SET_EVBIT EV_SYN: %w", err)
	}

	// Write the device description
	if err := writeUserDev(f, name); err != nil {
		f.Close()
		return nil, err
	}

	// Create the device
	if err := ioctl(f, uiDevCreate, 0); err != nil {
		f.Close()
		return nil, fmt.Errorf("UI_DEV_CREATE: %w", err)
	}

	time.Sleep(200 * time.Millisecond)

	return &VirtualDevice{
		file:       f,
		name:       name,
		deviceType: DeviceMouse,
	}, nil
}

// ---------------------------------------------------------------------------
// VirtualDevice methods
// ---------------------------------------------------------------------------

// Name returns the human-readable device name.
func (v *VirtualDevice) Name() string {
	return v.name
}

// Type returns the device type (keyboard or mouse).
func (v *VirtualDevice) Type() DeviceType {
	return v.deviceType
}

// SendKey sends a key press or release event. Use pressed=true for key-down
// and pressed=false for key-up. The code should be a Linux KEY_* constant
// (e.g. 30 for KEY_A).
func (v *VirtualDevice) SendKey(code uint16, pressed bool) error {
	val := int32(keyRelease)
	if pressed {
		val = keyPress
	}

	if err := v.writeEvent(evKey, code, val); err != nil {
		return fmt.Errorf("SendKey: %w", err)
	}

	return v.syncReport()
}

// MoveMouse sends a relative mouse movement event. dx is the horizontal
// delta (positive = right) and dy is the vertical delta (positive = down).
func (v *VirtualDevice) MoveMouse(dx, dy int32) error {
	if dx != 0 {
		if err := v.writeEvent(evRel, relX, dx); err != nil {
			return fmt.Errorf("MoveMouse REL_X: %w", err)
		}
	}
	if dy != 0 {
		if err := v.writeEvent(evRel, relY, dy); err != nil {
			return fmt.Errorf("MoveMouse REL_Y: %w", err)
		}
	}

	return v.syncReport()
}

// ClickButton sends a mouse button press or release. Common codes:
//
//	BTN_LEFT   = 0x110
//	BTN_RIGHT  = 0x111
//	BTN_MIDDLE = 0x112
func (v *VirtualDevice) ClickButton(code uint16, pressed bool) error {
	return v.SendKey(code, pressed)
}

// Close destroys the virtual device and releases the uinput file descriptor.
func (v *VirtualDevice) Close() error {
	if v.file == nil {
		return nil
	}

	// UI_DEV_DESTROY — ignore errors as the fd close will clean up anyway.
	_ = ioctl(v.file, uiDevDestroy, 0)

	err := v.file.Close()
	v.file = nil
	return err
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// writeEvent writes a single input_event to the uinput fd.
func (v *VirtualDevice) writeEvent(evType uint16, code uint16, value int32) error {
	now := time.Now()
	ev := inputEvent{
		Time: syscall.Timeval{
			Sec:  int64(now.Unix()),
			Usec: int64(now.UnixMicro() % 1e6),
		},
		Type:  evType,
		Code:  code,
		Value: value,
	}

	buf := make([]byte, unsafe.Sizeof(ev))
	// Use LittleEndian — uinput is defined by the host's native endianness
	// which on all Linux-supported architectures for Remote Studio is LE.
	binary.LittleEndian.PutUint64(buf[0:8], uint64(ev.Time.Sec))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(ev.Time.Usec))
	binary.LittleEndian.PutUint16(buf[16:18], ev.Type)
	binary.LittleEndian.PutUint16(buf[18:20], ev.Code)
	binary.LittleEndian.PutUint32(buf[20:24], uint32(ev.Value))

	_, err := v.file.Write(buf)
	return err
}

// syncReport sends an EV_SYN/SYN_REPORT to flush the event batch.
func (v *VirtualDevice) syncReport() error {
	return v.writeEvent(evSyn, synReport, 0)
}

// writeUserDev writes the uinput_user_dev struct to the file descriptor.
func writeUserDev(f *os.File, name string) error {
	var dev uinputUserDev
	copy(dev.Name[:], name)
	dev.ID = inputID{
		Bustype: busUSB,
		Vendor:  0x1234,
		Product: 0xFEED,
		Version: 1,
	}

	buf := make([]byte, unsafe.Sizeof(dev))
	// Name
	copy(buf[0:uinputMaxNameSize], dev.Name[:])
	offset := uinputMaxNameSize
	// input_id (4 x uint16 = 8 bytes)
	binary.LittleEndian.PutUint16(buf[offset:], dev.ID.Bustype)
	binary.LittleEndian.PutUint16(buf[offset+2:], dev.ID.Vendor)
	binary.LittleEndian.PutUint16(buf[offset+4:], dev.ID.Product)
	binary.LittleEndian.PutUint16(buf[offset+6:], dev.ID.Version)
	// The rest (EffectsMax + abs arrays) stays zero-initialized, which is correct.

	_, err := f.Write(buf)
	if err != nil {
		return fmt.Errorf("write uinput_user_dev: %w", err)
	}
	return nil
}

// ioctl performs a raw ioctl syscall on the given file.
func ioctl(f *os.File, request uintptr, val uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), request, val)
	if errno != 0 {
		return errno
	}
	return nil
}
