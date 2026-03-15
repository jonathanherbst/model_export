package blizzard

import (
	"reflect"
	"testing"
)

func TestBytesToUnsignedInts(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		arrayLen  int
		typeParam interface{} // dummy to indicate type
		expected  interface{}
	}{
		{
			name:      "uint8 single",
			data:      []byte{0x01},
			arrayLen:  1,
			typeParam: uint8(0),
			expected:  []uint64{1},
		},
		{
			name:      "uint8 multiple",
			data:      []byte{0x01, 0x02, 0xFF},
			arrayLen:  3,
			typeParam: uint8(0),
			expected:  []uint64{1, 2, 255},
		},
		{
			name:      "uint16",
			data:      []byte{0x01, 0x00, 0x02, 0x00},
			arrayLen:  2,
			typeParam: uint16(0),
			expected:  []uint64{1, 2},
		},
		{
			name:      "uint32",
			data:      []byte{0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00},
			arrayLen:  2,
			typeParam: uint32(0),
			expected:  []uint64{1, 2},
		},
		{
			name:      "uint64",
			data:      []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			arrayLen:  2,
			typeParam: uint64(0),
			expected:  []uint64{1, 2},
		},
		{
			name:      "uint8 single value",
			data:      []byte{0x01},
			arrayLen:  0,
			typeParam: uint8(0),
			expected:  uint64(1),
		},
		{
			name:      "uint16 single value",
			data:      []byte{0x01, 0x00},
			arrayLen:  0,
			typeParam: uint16(0),
			expected:  uint64(1),
		},
		{
			name:      "uint32 single value",
			data:      []byte{0x01, 0x00, 0x00, 0x00},
			arrayLen:  0,
			typeParam: uint32(0),
			expected:  uint64(1),
		},
		{
			name:      "uint64 single value",
			data:      []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			arrayLen:  0,
			typeParam: uint64(0),
			expected:  uint64(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}
			switch tt.typeParam.(type) {
			case uint8:
				result = bytesToUnsignedInts[uint8](tt.data, tt.arrayLen)
			case uint16:
				result = bytesToUnsignedInts[uint16](tt.data, tt.arrayLen)
			case uint32:
				result = bytesToUnsignedInts[uint32](tt.data, tt.arrayLen)
			case uint64:
				result = bytesToUnsignedInts[uint64](tt.data, tt.arrayLen)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("bytesToUnsignedInts() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBytesToSignedInts(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		arrayLen  int
		typeParam interface{} // dummy to indicate type
		expected  interface{}
	}{
		{
			name:      "int8 positive",
			data:      []byte{0x01, 0x7F},
			arrayLen:  2,
			typeParam: int8(0),
			expected:  []int64{1, 127},
		},
		{
			name:      "int8 negative",
			data:      []byte{0xFF, 0x80},
			arrayLen:  2,
			typeParam: int8(0),
			expected:  []int64{-1, -128},
		},
		{
			name:      "int16",
			data:      []byte{0x01, 0x00, 0xFF, 0xFF},
			arrayLen:  2,
			typeParam: int16(0),
			expected:  []int64{1, -1},
		},
		{
			name:      "int32",
			data:      []byte{0x01, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF},
			arrayLen:  2,
			typeParam: int32(0),
			expected:  []int64{1, -1},
		},
		{
			name:      "int64",
			data:      []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			arrayLen:  2,
			typeParam: int64(0),
			expected:  []int64{1, -1},
		},
		{
			name:      "int8 single value",
			data:      []byte{0x01},
			arrayLen:  0,
			typeParam: int8(0),
			expected:  int64(1),
		},
		{
			name:      "int16 single value",
			data:      []byte{0x01, 0x00},
			arrayLen:  0,
			typeParam: int16(0),
			expected:  int64(1),
		},
		{
			name:      "int32 single value",
			data:      []byte{0x01, 0x00, 0x00, 0x00},
			arrayLen:  0,
			typeParam: int32(0),
			expected:  int64(1),
		},
		{
			name:      "int64 single value",
			data:      []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			arrayLen:  0,
			typeParam: int64(0),
			expected:  int64(1),
		},
		{
			name:      "int8 negative single value",
			data:      []byte{0xFF},
			arrayLen:  0,
			typeParam: int8(0),
			expected:  int64(-1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}
			switch tt.typeParam.(type) {
			case int8:
				result = bytesToSignedInts[int8](tt.data, tt.arrayLen)
			case int16:
				result = bytesToSignedInts[int16](tt.data, tt.arrayLen)
			case int32:
				result = bytesToSignedInts[int32](tt.data, tt.arrayLen)
			case int64:
				result = bytesToSignedInts[int64](tt.data, tt.arrayLen)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("bytesToSignedInts() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBytesToInts_InvalidLength(t *testing.T) {
	// Test invalid length (byte array size does not match expected count)
	data := []byte{0x01, 0x00, 0x00} // 3 bytes, but arrayLen=1 expects 4 bytes for uint32

	assertPanics(t, func() {
		bytesToUnsignedInts[uint32](data, 1)
	})

	// Valid length should work
	data2 := []byte{0x01, 0x00, 0x00, 0x00}
	result := bytesToUnsignedInts[uint32](data2, 1).([]uint64)
	expected := []uint64{1}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("bytesToUnsignedInts[uint32]() = %v, want %v", result, expected)
	}
}

func assertPanics(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	f()
}
