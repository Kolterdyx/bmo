package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// Linux Event Types and Codes
type EventType int

const (
	EvSyn EventType = 0x00
	EvKey           = 0x01
	EvAbs           = 0x03
)

type EventCode int

const (
	AbsX     EventCode = 0x00
	AbsY               = 0x01
	BtnTouch           = 0x14a // 330 in decimal
)

type InputEvent struct {
	Timestamp time.Time
	Type      EventType
	Code      EventCode
	Value     int64
}

func processInput(i io.Reader) {

	ev, err := readEvent(i)
	if err != nil {
		fmt.Printf("Error reading input event: %v\n", err)
		return
	}
	fmt.Printf("Input event: %+v\n", ev)

}

func readEvent(f io.Reader) (InputEvent, error) {
	var e InputEvent

	b := make([]byte, 24)
	if _, err := f.Read(b); err != nil {
		return InputEvent{}, fmt.Errorf("reading input event: %w", err)
	}
	sec := binary.LittleEndian.Uint64(b[0:8])
	usec := binary.LittleEndian.Uint64(b[8:16])
	e.Timestamp = time.Unix(int64(sec), int64(usec))
	e.Type = EventType(binary.LittleEndian.Uint16(b[16:18]))
	e.Code = EventCode(binary.LittleEndian.Uint16(b[18:20]))
	var value int32
	if err := binary.Read(bytes.NewReader(b[20:]), binary.LittleEndian, &value); err != nil {
		return InputEvent{}, fmt.Errorf("reading input event value: %w", err)
	}
	e.Value = int64(value)
	return e, nil
}
