package model

import (
	"github.com/go-gl/mathgl/mgl32"
)

type Model struct {
	VertexPositions   []mgl32.Vec3
	VertexNormals     []mgl32.Vec3
	VertexBones       [][4]uint8
	VertexBoneWeights [][4]uint8
	Skin              *Skin
	Skeleton          *Skeleton
	Animations        []Animation
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
	BoneParents         []int
	BoneInvBindMatrices []mgl32.Mat4
}

type Animation struct {
	Name              string
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
