package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sync"
	"time"
)

type EventType uint16

const (
	EvSyn EventType = 0x00
	EvKey EventType = 0x01
	EvAbs EventType = 0x03
)

func (e EventType) String() string {
	switch e {
	case EvSyn:
		return "EvSyn"
	case EvKey:
		return "EvKey"
	case EvAbs:
		return "EvAbs"
	default:
		return fmt.Sprintf("Unknown event type: %d", e)
	}
}

type EventCode uint16

const (
	AbsX        EventCode = 0
	AbsY        EventCode = 1
	AbsPressure EventCode = 24
	BtnTouch    EventCode = 330
)

func (e EventCode) String() string {
	switch e {
	case AbsX:
		return "AbsX"
	case AbsY:
		return "AbsY"
	case AbsPressure:
		return "AbsPressure"
	case BtnTouch:
		return "BtnTouch"
	default:
		return fmt.Sprintf("Unknown event code: %d", e)
	}
}

type InputEvent struct {
	Timestamp time.Time
	Type      EventType
	Code      EventCode
	Value     int32
}

func (e InputEvent) String() string {
	return fmt.Sprintf("InputEvent{Timestamp: %v, Type: %s, Code: %s, Value: %d}", e.Timestamp, e.Type, e.Code, e.Value)
}

var (
	touchMu        sync.Mutex
	touchActive    bool
	touchX, touchY int
)

const (
	touchWidth  = 4095.
	touchHeight = 4095.
)

func getTouchState() (active bool, x, y int) {
	touchMu.Lock()
	defer touchMu.Unlock()
	if touchActive {
		fmt.Printf("Getting touch state: x=%d, y=%d\n", touchX, touchY)
	}
	return touchActive, touchX, touchY
}

func processInput(r io.Reader) error {
	fmt.Printf("Processing input events from %T\n", r)
	ev, err := readEvent(r)
	if err != nil {
		return err
	}
	touchMu.Lock()
	defer touchMu.Unlock()
	fmt.Printf("Input event: %s\n", ev)
	switch ev.Type {
	case EvAbs:
		switch ev.Code {
		case AbsX:
			touchY = int(math.Floor((float64(ev.Value) / touchWidth) * float64(width)))
		case AbsY:
			touchX = height - int(math.Floor((float64(ev.Value)/touchHeight)*float64(height)))
		case AbsPressure:
			// Handle pressure value if needed
		}
	case EvKey:
		if ev.Code == BtnTouch {
			touchActive = ev.Value != 0
		}
	}
	return nil
}

// readEvent reads one linux input_event from r.
// Layout: timeval (sec uint64 + usec uint64), type uint16, code uint16, value int32 — 24 bytes total.
func readEvent(r io.Reader) (InputEvent, error) {
	var b [24]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return InputEvent{}, fmt.Errorf("reading input event: %w", err)
	}
	sec := binary.LittleEndian.Uint64(b[0:8])
	usec := binary.LittleEndian.Uint64(b[8:16])
	return InputEvent{
		Timestamp: time.Unix(int64(sec), int64(usec)*1000),
		Type:      EventType(binary.LittleEndian.Uint16(b[16:18])),
		Code:      EventCode(binary.LittleEndian.Uint16(b[18:20])),
		Value:     int32(binary.LittleEndian.Uint32(b[20:24])),
	}, nil
}
