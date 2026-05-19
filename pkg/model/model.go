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
	Configurations    []ConfigurationComponent
	Materials         []Material
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

type ConfigurationChoice struct {
	Name       string
	Color      uint32
	OrderIndex uint
	OptionName string
}

type ConfigurationComponent struct {
	Configurations   []ConfigurationChoice
	Geosets          []int
	TextureFragments []TextureFragment
}

type Material struct {
	Width  uint
	Height uint
}

type TextureFragment struct {
	MaterialIdx int
	Img         int
	X           uint
	Y           uint
	Width       uint
	Height      uint
	Layer       uint
	BlendMode   BlendMode
}

type BlendMode int

const (
	BlendModeNone BlendMode = iota
	BlendModeInferAlphaBlend
	BlendModeAlphaStraight
)
