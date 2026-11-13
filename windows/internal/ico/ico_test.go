package ico

import (
	"encoding/binary"
	"image"
	"testing"
)

func TestEncodeHeader(t *testing.T) {
	data, err := Encode([]image.Image{image.NewNRGBA(image.Rect(0, 0, 16, 16)), image.NewNRGBA(image.Rect(0, 0, 256, 256))})
	if err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	if le.Uint16(data[2:]) != 1 || le.Uint16(data[4:]) != 2 {
		t.Fatalf("bad header % x", data[:6])
	}
	if data[6] != 16 || data[6+16] != 0 { // 256 is stored as 0
		t.Fatalf("bad sizes %d %d", data[6], data[22])
	}
	off := le.Uint32(data[6+12:])
	n := le.Uint32(data[6+8:])
	if int(off+n) > len(data) || le.Uint32(data[off:]) != 40 {
		t.Fatalf("bad first entry offset=%d size=%d", off, n)
	}
	// 16x16x4 pixels + 16 rows of 4-byte mask + 40-byte header
	if n != 40+16*16*4+16*4 {
		t.Fatalf("entry size %d", n)
	}
}

func TestTrayIconsForEveryState(t *testing.T) {
	if len(trayImages) == 0 {
		t.Fatal("no embedded tray images")
	}
	for s := Ready; s <= Problem; s++ {
		if len(TrayIcon(s)) < 1000 {
			t.Fatalf("state %d: icon too small", s)
		}
	}
}
