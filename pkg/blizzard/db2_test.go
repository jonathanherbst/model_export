package blizzard

import "testing"

func TestBitBuffer(t *testing.T) {
	data := []byte{0x01, 0x23, 0x45, 0x67}
	bb := newWDC5BitBuffer(data)

	unsigned_tests := []struct {
		name        string
		index, size uint
		expected    uint64
	}{
		{"part of a byte", 1, 7, 0},
		{"one full byte", 8, 8, 0x23},
		{"two full bytes", 8, 16, 0x4523},
		{"bits accross byte boundary", 7, 10, 582},
	}
	for _, tt := range unsigned_tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bb.GetUnsigned(tt.index, tt.size)
			if result != tt.expected {
				t.Errorf("GetUnsigned(%d, %d) = %d; expected %d", tt.index, tt.size, result, tt.expected)
			}
		})
	}

	signed_tests := []struct {
		name        string
		index, size uint
		expected    int64
	}{
		{"negative", 0, 10, -255},
		{"positive", 8, 8, 0x23},
	}
	for _, tt := range signed_tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bb.GetSigned(tt.index, tt.size)
			if result != tt.expected {
				t.Errorf("GetUnsigned(%d, %d) = %d; expected %d", tt.index, tt.size, result, tt.expected)
			}
		})
	}
}
