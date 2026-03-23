package blizzard

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func M2SkinFromFile(path string) (*M2Skin, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read skin file: %w", err)
	}
	defer f.Close()

	buf, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read skin file: %w", err)
	}
	return M2SkinFromBuf(buf)
}

func M2SkinFromBuf(buf []byte) (*M2Skin, error) {
	var header M2SkinHeader
	if _, err := binary.Decode(buf, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("failed to read skin file: %w", err)
	}
	if string(header.Magic[:]) != "SKIN" {
		return nil, fmt.Errorf("invalid skin file")
	}

	var skin M2Skin
	vertices := header.Verticies.MakeArray(buf, 0)
	indices := header.Indicies.MakeArray(buf, 0)
	skin.Meshes = make([]M2Mesh, header.Submeshes.Size)
	for i, submesh := range header.Submeshes.MakeArray(buf, 0) {
		skin.Meshes[i].Id = submesh.SkinSectionId
		skin.Meshes[i].VertexIndexes = make([]uint32, submesh.IndexCount)
		for j := 0; j < int(submesh.IndexCount); j++ {
			index := int(submesh.Level)<<16 | int(submesh.IndexStart)
			skin.Meshes[i].VertexIndexes[j] = uint32(vertices[indices[index+j]])
		}
	}

	return &skin, nil
}

type M2Skin struct {
	Meshes []M2Mesh
}

type M2Mesh struct {
	Id            uint16 //
	VertexIndexes []uint32
}

type M2SkinHeader struct {
	Magic         [4]byte
	Verticies     m2Array[uint16]
	Indicies      m2Array[uint16]
	Bones         m2Array[[4]byte]
	Submeshes     m2Array[M2SkinSection]
	Batches       m2Array[M2Batch]
	BoneCountMax  uint32
	ShadowBatches m2Array[M2ShadowBatch]
}

type M2SkinSection struct {
	SkinSectionId      uint16
	Level              uint16 // (level << 16) is added (|ed) to startTriangle and alike to avoid having to increase those fields to uint32s.
	VertexStart        uint16 // Starting vertex number.
	VertexCount        uint16 // Number of vertices.
	IndexStart         uint16 // Starting triangle index (that's 3* the number of triangles drawn so far)
	IndexCount         uint16 // Number of triangle indices.
	BoneCount          uint16 // Number of elements in the bone lookup table. Max seems to be 256 in Wrath. Shall be ≠ 0.
	BoneComboIndex     uint16 // Starting index in the bone lookup table.
	BoneInfluences     uint16
	CenterBoneIndex    uint16
	CenterPosition     C3Vector // Average position of all the vertices in the sub mesh.
	SortCenterPosition C3Vector // The center of the box when an axis aligned box is built around the vertices in the submesh.
	SortRadius         float32  // Distance of the vertex farthest from CenterBoundingBox.
}

type M2Batch struct {
	Flags                      uint8
	PriorityPlane              int8
	ShaderId                   uint16
	SkinSectionIndex           uint16
	GeosetIndex                uint16
	ColorIndex                 uint16
	MaterialIndex              uint16
	MaterialLayer              uint16
	TextureCount               uint16
	TextureComboIndex          uint16
	TextureCoordComboIndex     uint16
	TextureWeightComboIndex    uint16
	TextureTransformComboIndex uint16
}

type M2ShadowBatch struct {
	Flags          uint8
	Flags2         uint8
	Unknown1       uint16
	SubmeshId      uint16
	TextureId      uint16
	ColorId        uint16
	TransparencyId uint16
}
