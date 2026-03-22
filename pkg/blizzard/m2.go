package blizzard

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func M2FromFile(path string) (*M2, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Failed reading M2: %w", err)
	}
	return M2FromReader(f)
}

func M2FromReader(r io.Reader) (*M2, error) {
	var m2 M2
	m2r := M2Reader{r}
	for header, data := range m2r.Chunks {
		switch string(header.Token[:]) {
		case "MD21":
			md20, err := M2MD20FromChunk(header, data)
			if err != nil {
				return nil, fmt.Errorf("Failed reading M2: %w", err)
			}
			m2.Vertices = md20.Verticies()
		}
	}
	return &m2, nil
}

type M2 struct {
	Vertices []M2Vertex
}

func M2MD20FromChunk(header m2ChunkHeader, data []byte) (*M2MD20, error) {
	if string(header.Token[:]) == "MD21" {
		if _, err := binary.Decode(data, binary.LittleEndian, &header); err != nil {
			return nil, fmt.Errorf("Failed parsing md20 header: %w", err)
		}
		if string(header.Token[:]) != "MD20" {
			panic("MD21 chunk without a MD20 inside")
		}
		data = data[8:]
	} else if string(header.Token[:]) != "MD20" {
		return nil, fmt.Errorf("Don't know how to get MD20 from %s chunk", string(header.Token[:]))
	}

	var md20Header m2MD20Header
	if _, err := binary.Decode(data, binary.LittleEndian, &md20Header); err != nil {
		return nil, fmt.Errorf("Failed parsing md20 header: %w", err)
	}
	return &M2MD20{header, md20Header, data}, nil
}

type M2MD20 struct {
	Header     m2ChunkHeader
	MD20Header m2MD20Header
	data       []byte
}

func (md20 M2MD20) Verticies() []M2Vertex {
	return md20.MD20Header.Vertices.MakeArray(md20)
}

func (md20 M2MD20) offset(arrayOffset uint32) int {
	return int(arrayOffset) - 8
}

type M2Reader struct {
	Reader io.Reader
}

func (r M2Reader) Chunks(yield func(m2ChunkHeader, []byte) bool) {
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

type m2MD20Header struct {
	Name                   m2Array[byte]
	GlobalFlags            uint32
	GlobalLoops            m2Array[M2Loop]
	Sequences              m2Array[M2Sequence]
	SequenceIdxHashById    m2Array[uint16]
	Bones                  m2Array[M2CompBone]
	BoneIndicesById        m2Array[uint16]
	Vertices               m2Array[M2Vertex]
	NumSkinProfiles        uint32
	Colors                 m2Array[M2Color]
	Textures               m2Array[M2Texture]
	TextureWeights         m2Array[M2TextureWeight]
	TextureTransforms      m2Array[M2TextureTransform]
	TextureIndicesById     m2Array[uint16]
	Materials              m2Array[M2Material]
	BoneCombos             m2Array[uint16]
	TextureCombos          m2Array[uint16]
	TextureCoordCombos     m2Array[uint16]
	TextureWeightCombos    m2Array[uint16]
	TextureTransformCombos m2Array[uint16]
	BoundingBox            CAaBox
	BoundingSphereRadius   float32
	CollisionBox           CAaBox
	CollisionSphereRadius  float32
	CollisionIndices       m2Array[uint16]
	CollisionPositions     m2Array[C3Vector]
	CollisionFaceNormals   m2Array[C3Vector]
	Attachments            m2Array[M2Attachment]
	AttachmentIndicesById  m2Array[uint16]
	Events                 m2Array[M2Event]
	Lights                 m2Array[M2Light]
	Cameras                m2Array[M2Camera]
	CameraIndicesById      m2Array[uint16]
	RibbonEmitters         m2Array[M2Ribbon]
	ParticleEmitters       m2Array[M2Particle]
	// Optional: TextureCombinerCombos M2Array[uint16] // if flag_use_texture_combiner_combos
}

type C3Vector struct {
	X, Y, Z float32
}

type C2Vector struct {
	X, Y float32
}

type CAaBox struct {
	Min, Max C3Vector
}

type m2Array[T any] struct {
	Size, Offset uint32
}

func (arr m2Array[T]) MakeArray(md20 M2MD20) []T {
	data := md20.data[md20.offset(arr.Offset):]
	output := make([]T, arr.Size)
	size, err := binary.Decode(data, binary.LittleEndian, output)
	if err != nil {
		return nil
	}

	var t T
	if binary.Size(&t)*int(arr.Size) != size {
		panic("didn't get the entire array")
	}

	return output
}

type M2Loop struct {
	Timestamp uint32
}

type M2Sequence struct {
	// Placeholder - define fields as needed
}

type M2CompBone struct {
	// Placeholder
}

type M2Vertex struct {
	Pos         C3Vector
	BoneWeights [4]uint8
	BoneIndices [4]uint8
	Normal      C3Vector
	TexCoords   C2Vector
	// Placeholder
}

type M2Color struct {
	// Placeholder
}

type M2Texture struct {
	// Placeholder
}

type M2TextureWeight struct {
	// Placeholder
}

type M2TextureTransform struct {
	// Placeholder
}

type M2Material struct {
	// Placeholder
}

type M2Attachment struct {
	// Placeholder
}

type M2Event struct {
	// Placeholder
}

type M2Light struct {
	// Placeholder
}

type M2Camera struct {
	// Placeholder
}

type M2Ribbon struct {
	// Placeholder
}

type M2Particle struct {
	// Placeholder
}
