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
	defer f.Close()

	return M2FromReader(f)
}

func M2FromReader(r io.Reader) (*M2, error) {
	var m2 M2
	m2r := M2ChunkReader{r}
	nSkins := 0
	for header, data := range m2r.Chunks {
		switch string(header.Token[:]) {
		case "MD21":
			md20, err := M2MD20FromChunk(header, data)
			if err != nil {
				return nil, fmt.Errorf("Failed reading M2: %w", err)
			}
			m2.Vertices = md20.Verticies()
			bones := md20.Bones()
			m2.Bones = make([]m2LoadedBone, len(bones))
			for i, bone := range bones {
				m2.Bones[i] = bone.Load(data, 0)
			}
			m2.Sequences = md20.Sequences()
			nSkins = int(md20.MD20Header.NumSkinProfiles)
		case "SFID":
			m2.SkinFileIds = make([]uint32, nSkins)
			binary.Decode(data, binary.LittleEndian, m2.SkinFileIds)
			// the rest is lod file ids (smaller meshes when rendering farther away?)
		case "SKID":
			m2.SkelFileIds = make([]uint32, len(data)/4)
			binary.Decode(data, binary.LittleEndian, m2.SkelFileIds)
		}
	}
	return &m2, nil
}

type M2 struct {
	Vertices    []M2Vertex
	Bones       []m2LoadedBone
	Sequences   []M2Sequence
	SkinFileIds []uint32
	SkelFileIds []uint32
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
	vertices := md20.MD20Header.Vertices.Load(md20.data, -8)
	for _, v := range vertices {
		// blizzard doesn't normalize its normals :)
		v.Normal.Normalize()
	}
	return vertices
}

func (md20 M2MD20) Bones() []m2CompBone {
	return md20.MD20Header.Bones.Load(md20.data, -8)
}

func (md20 M2MD20) Sequences() []M2Sequence {
	return md20.MD20Header.Sequences.Load(md20.data, -8)
}

type m2MD20Header struct {
	Name                   m2Array[byte]
	GlobalFlags            uint32
	GlobalLoops            m2Array[M2Loop]
	Sequences              m2Array[M2Sequence]
	SequenceIdxHashById    m2Array[uint16]
	Bones                  m2Array[m2CompBone]
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
