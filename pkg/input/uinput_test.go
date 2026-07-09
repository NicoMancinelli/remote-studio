package input

import "testing"

func TestVirtualDeviceNameAndType(t *testing.T) {
	cases := []struct {
		name       string
		devName    string
		devType    DeviceType
		wantName   string
		wantType   DeviceType
		wantTypeIs KeyboardMouse // human-readable
	}{
		{
			name:     "keyboard",
			devName:  "MyVirtualKB",
			devType:  DeviceKeyboard,
			wantName: "MyVirtualKB",
			wantType: DeviceKeyboard,
		},
		{
			name:     "mouse",
			devName:  "MyVirtualMouse",
			devType:  DeviceMouse,
			wantName: "MyVirtualMouse",
			wantType: DeviceMouse,
		},
		{
			name:     "empty name is preserved",
			devName:  "",
			devType:  DeviceKeyboard,
			wantName: "",
			wantType: DeviceKeyboard,
		},
		{
			name:     "long name (truncated on uinput side, but struct keeps full)",
			devName:  "ThisNameIsMuchLongerThan80BytesAndWouldBeTruncatedByTheKernelButTheStructStoresItUnchanged",
			devType:  DeviceMouse,
			wantName: "ThisNameIsMuchLongerThan80BytesAndWouldBeTruncatedByTheKernelButTheStructStoresItUnchanged",
			wantType: DeviceMouse,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := &VirtualDevice{
				name:       c.devName,
				deviceType: c.devType,
				// file is nil — fine for testing Name/Type, those don't
				// touch the file descriptor.
			}
			if got := v.Name(); got != c.wantName {
				t.Errorf("Name() = %q, want %q", got, c.wantName)
			}
			if got := v.Type(); got != c.wantType {
				t.Errorf("Type() = %v, want %v", got, c.wantType)
			}
		})
	}
}

func TestDeviceTypeConstants(t *testing.T) {
	// DeviceKeyboard and DeviceMouse are public iota constants. Verify
	// they have stable values — these may be persisted in user state
	// (e.g. session files), so accidentally renumbering them would
	// break upgrades.
	if DeviceKeyboard != 0 {
		t.Errorf("DeviceKeyboard = %d, want 0", DeviceKeyboard)
	}
	if DeviceMouse != 1 {
		t.Errorf("DeviceMouse = %d, want 1", DeviceMouse)
	}
}

func TestSendKeyValueMapping(t *testing.T) {
	// SendKey's pressed=true → value=keyPress(1); pressed=false → value=keyRelease(0).
	// The function dispatches to writeEvent which calls ioctl, but the
	// value-to-pressed mapping is the part we can verify by extracting
	// it (or by mocking). Without mocking, we verify the constants:
	if keyPress != 1 {
		t.Errorf("keyPress = %d, want 1", keyPress)
	}
	if keyRelease != 0 {
		t.Errorf("keyRelease = %d, want 0", keyRelease)
	}
	// Linux input event types we rely on.
	if evKey != 0x01 {
		t.Errorf("evKey = %#x, want 0x01 (linux input subsystem EV_KEY)", evKey)
	}
	if evRel != 0x02 {
		t.Errorf("evRel = %#x, want 0x02 (EV_REL)", evRel)
	}
	if evSyn != 0x00 {
		t.Errorf("evSyn = %#x, want 0x00 (EV_SYN)", evSyn)
	}
	if synReport != 0x00 {
		t.Errorf("synReport = %#x, want 0x00 (SYN_REPORT)", synReport)
	}
}

func TestUinputPath(t *testing.T) {
	// uinputPath is the Linux device path. Catch a typo / platform
	// change here.
	if uinputPath != "/dev/uinput" {
		t.Errorf("uinputPath = %q, want \"/dev/uinput\"", uinputPath)
	}
}

func TestConstantsLayout(t *testing.T) {
	// Sanity-check that the ioctl numbers are non-zero and in the
	// expected ballpark (top bits are direction + size).
	// UI_SET_EVBIT = 0x40045564 — direction=01 (write), size=14, type='U', nr=0x64.
	for _, c := range []struct {
		name string
		got  uintptr
		min  uintptr
	}{
		{"uiSetEvbit", uintptr(uiSetEvbit), 0x40000000},
		{"uiSetKeybit", uintptr(uiSetKeybit), 0x40000000},
		{"uiSetRelbit", uintptr(uiSetRelbit), 0x40000000},
		{"uiSetAbsbit", uintptr(uiSetAbsbit), 0x40000000},
		{"uiDevCreate", uintptr(uiDevCreate), 0x00005501}, // _IOWR
		{"uiDevDestroy", uintptr(uiDevDestroy), 0x00005502},
	} {
		if c.got < c.min {
			t.Errorf("%s = %#x, expected >= %#x (looks like an ioctl bit pattern got mangled)", c.name, c.got, c.min)
		}
	}
}

// Touch the uinputMaxNameSize and other typed-only constants so they
// don't drift unused.
func TestUinputTypeConstantsInUse(t *testing.T) {
	if uinputMaxNameSize <= 0 {
		t.Error("uinputMaxNameSize should be > 0")
	}
	_ = relX
	_ = relY
	_ = busUSB
}

// KeyboardMouse is just a comment-typed alias used in the test table
// above for documentation; not a real type.
type KeyboardMouse = string
