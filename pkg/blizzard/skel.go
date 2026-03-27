package blizzard

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type M2Skeleton struct {
	Name             string
	Bones            []m2LoadedBone
	Sequences        []M2Sequence
	SequenceIds      []uint16
	ParentSkelFileId *uint32
	AnimMeta         []m2AFIDData
	BoneFileIds      []m2BFIDData
}

func M2SkelFromFile(path string) (*M2Skeleton, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open skel file: %w", err)
	}
	return M2SkelFromReader(file)
}

func M2SkelFromReader(r io.Reader) (*M2Skeleton, error) {
	var skel M2Skeleton

	reader := M2ChunkReader{Reader: r}
	for header, data := range reader.Chunks {
		switch string(header.Token[:]) {
		case "SKL1":
			var header m2SKL1Header
			if _, err := binary.Decode(data, binary.LittleEndian, &header); err != nil {
				return nil, fmt.Errorf("failed to decode SKL1: %w", err)
			}
			skel.Name = string(header.Name.Load(data, 0))
		case "SKB1":
			var header m2SKB1Header
			if _, err := binary.Decode(data, binary.LittleEndian, &header); err != nil {
				return nil, fmt.Errorf("failed to decode SKB1: %w", err)
			}
			bones := header.Bones.Load(data, 0)
			skel.Bones = make([]m2LoadedBone, len(bones))
			for i, bone := range bones {
				skel.Bones[i] = bone.Load(data, 0)
			}
		case "SKS1":
			var header m2SKS1Header
			if _, err := binary.Decode(data, binary.LittleEndian, &header); err != nil {
				return nil, fmt.Errorf("failed to decode SKS1: %w", err)
			}
			skel.Sequences = header.Sequences.Load(data, 0)
			skel.SequenceIds = header.SequenceLookup.Load(data, 0)
		case "SKPD":
			var header m2SKPDHeader
			if _, err := binary.Decode(data, binary.LittleEndian, &header); err != nil {
				return nil, fmt.Errorf("failed to decode SKPD: %w", err)
			}
			skel.ParentSkelFileId = &header.ParentSkelFileId
		case "AFID":
			var scratch m2AFIDData
			skel.AnimMeta = make([]m2AFIDData, len(data)/binary.Size(scratch))
			if _, err := binary.Decode(data, binary.LittleEndian, skel.AnimMeta); err != nil {
				return nil, fmt.Errorf("failed to decode AFID: %w", err)
			}
		case "BFID":
			var scratch m2BFIDData
			skel.BoneFileIds = make([]m2BFIDData, len(data)/binary.Size(scratch))
			if _, err := binary.Decode(data, binary.LittleEndian, skel.BoneFileIds); err != nil {
				return nil, fmt.Errorf("failed to decode BFID: %w", err)
			}
		}
	}

	return &skel, nil
}
