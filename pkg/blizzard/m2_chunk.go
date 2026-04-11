package blizzard

import (
	"encoding/binary"
	"io"
)

type M2ChunkReader struct {
	Reader io.Reader
}

func (r M2ChunkReader) Chunks(yield func(m2ChunkHeader, []byte) bool) {
	var header m2ChunkHeader
	if err := binary.Read(r.Reader, binary.LittleEndian, &header); err != nil {
		return
	}

	// if the first header token is "MD20" the file is not chunked and instead has MD20 data.
	if string(header.Token[:]) == "MD20" {
		data, err := io.ReadAll(r.Reader)
		if err != nil {
			return
		}
		yield(header, data)
		return
	}

	for {
		data, err := io.ReadAll(io.LimitReader(r.Reader, int64(header.SizeOrVersion)))
		if err != nil {
			return
		}

		if !yield(header, data) {
			return
		}

		if err = binary.Read(r.Reader, binary.LittleEndian, &header); err != nil {
			return
		}
	}
}

type m2ChunkHeader struct {
	Token         [4]byte
	SizeOrVersion uint32
}

type m2AFIDData struct {
	AnimId     uint16
	SubAnimId  uint16
	AnimFileId uint32
}

type m2BFIDData struct {
	BoneFileId uint32
}

type m2SKL1Header struct {
	Flags   uint32 // always 0x100
	Name    m2Array[byte]
	Padding [4]byte // no clue what this is
}

type m2SKS1Header struct {
	GlobalLoops    m2Array[M2Loop]
	Sequences      m2Array[M2Sequence]
	SequenceLookup m2Array[int16]
	Padding        [8]byte
}

type m2SKB1Header struct {
	Bones      m2Array[m2CompBone]
	BoneLookup m2Array[uint16]
}

type m2SKA1Header struct {
	Attachments      m2Array[M2Attachment]
	AttachmentLookup m2Array[uint16]
}

type m2SKPDHeader struct {
	_0x00            [8]byte
	ParentSkelFileId uint32 // this is an id of another skeleton file to append to to make the full skeleton
	_0x0c            [4]byte
}
