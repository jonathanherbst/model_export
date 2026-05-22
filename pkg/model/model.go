package model

import (
	"image"

	"github.com/go-gl/mathgl/mgl32"
)

type Model struct {
	VertexPositions   []mgl32.Vec3
	VertexNormals     []mgl32.Vec3
	VertexBones       [][4]uint8
	VertexBoneWeights [][4]uint8
	VertexTexCoords_0 []mgl32.Vec2
	VertexTexCoords_1 []mgl32.Vec2
	Skin              *Skin
	Skeleton          *Skeleton
	Animations        []Animation
	Choices           []ConfigChoice
	Elements          []ConfigElement
	SegmentedTextures []Texture
	Images            []image.Image
}

type RenderProcess int

const (
	RenderTriangles RenderProcess = iota
)

type Skin struct {
	Meshes []Mesh
}

type Mesh struct {
	Name          string
	IsEquipment   bool
	VertexMap     []int
	RenderProcess RenderProcess
}

type Skeleton struct {
	Name                string
	BoneNames           []string
	BoneParents         []int
	BoneInvBindMatrices []mgl32.Mat4
}

type Animation struct {
	Name              string
	Duration          float32
	TranslationTracks []AnimationTrack[mgl32.Vec3]
	RotationTracks    []AnimationTrack[mgl32.Vec4]
	ScaleTracks       []AnimationTrack[mgl32.Vec3]
}

type AnimationTrack[T any] struct {
	Bone          int
	Interpolation InterpolationType
	Timestamps    []float32
	Values        []T
}

type InterpolationType int

const (
	InterpolationStep InterpolationType = iota
	InterpolationLinear
	InterpolationCubicBezier
	InterpolationCubicHermite
)

type TextureSegment struct {
	X         uint   `json:"x"`
	Y         uint   `json:"y"`
	Width     uint   `json:"width"`
	Height    uint   `json:"height"`
	BlendMode string `json:"blend_mode"`
}

type Texture struct {
	Width    uint             `json:"width"`
	Height   uint             `json:"height"`
	Segments []TextureSegment `json:"segments"`
}

type ConfigChoice struct {
	Option string `json:"option"`
	Choice string `json:"choice"`
	Color  uint32 `json:"color"`
}

type ElementMaterial struct {
	MaterialIdx int `json:"material"`
	SegmentIdx  int `json:"segment"`
	ImageIdx    int `json:"image"`
}

type ConfigElement struct {
	ChoiceIdxes []int             `json:"choices"`
	Materials   []ElementMaterial `json:"materials"`
	MeshIdxes   []int             `json:"meshes"`
}
