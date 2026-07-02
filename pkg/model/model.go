package model

import (
	"image"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/qmuntal/gltf"
	"golang.org/x/image/draw"
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
	BoundingBox       *[2]mgl32.Vec3
	Animations        []Animation
	Choices           []ConfigChoice
	Elements          []ConfigElement
	Materials         []Material
	Images            []image.Image
}

func (mdl Model) GetDefaultElements() []ConfigElement {
	selectedOptions := make(map[string]int)
	for i, choice := range mdl.Choices {
		if _, ok := selectedOptions[choice.Option]; !ok {
			selectedOptions[choice.Option] = i
		}
	}

	selectedElements := make([]ConfigElement, 0)
	for _, element := range mdl.Elements {
		selected := true
		for _, choiceIdx := range element.ChoiceIdxes {
			selected = selected && (selectedOptions[mdl.Choices[choiceIdx].Option] == choiceIdx)
		}
		if selected {
			selectedElements = append(selectedElements, element)
		}
	}

	return selectedElements
}

func (mdl Model) MakeDefaultTextures() []image.Image {
	elements := mdl.GetDefaultElements()

	mats := make([]ConfigMaterial, 0)
	for _, element := range elements {
		for _, mat := range element.Materials {
			mats = append(mats, mat)
		}
	}

	textures := make([]image.Image, len(mdl.Materials))
	for matIdx, tex := range mdl.Materials {
		if tex.SegmentedTexture != nil {
			texture := image.NewRGBA(image.Rect(0, 0, int(tex.SegmentedTexture.Width), int(tex.SegmentedTexture.Height)))
			for _, mat := range mats {
				if mat.MaterialIdx == matIdx {
					seg := tex.SegmentedTexture.Segments[mat.SegmentIdx]
					texImg := mdl.Images[mat.ImageIdx]
					// need to resize the image to the fragment size
					img := image.NewRGBA(image.Rect(0, 0, int(seg.Width), int(seg.Height)))
					draw.BiLinear.Scale(img, img.Rect, texImg, texImg.Bounds(), draw.Over, nil)
					rect := img.Bounds().Add(image.Point{int(seg.X), int(seg.Y)})
					draw.Draw(texture, rect, img, image.Point{0, 0}, draw.Over)
				}
			}
			textures[matIdx] = texture
		} else {
			textures[matIdx] = mdl.Images[*tex.ImageIdx]
		}
	}

	return textures
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
	IsStatic      bool
	VertexMap     []int
	MaterialName  string
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

type WrappingMode gltf.WrappingMode

const (
	WrapRepeat         WrappingMode = WrappingMode(gltf.WrapRepeat)
	WrapClampToEdge                 = WrappingMode(gltf.WrapClampToEdge)
	WrapMirroredRepeat              = WrappingMode(gltf.WrapMirroredRepeat)
)

type AlphaMode gltf.AlphaMode

const (
	AlphaOpaque AlphaMode = AlphaMode(gltf.AlphaOpaque)
	AlphaMask             = AlphaMode(gltf.AlphaMask)
	AlphaBlend            = AlphaMode(gltf.AlphaBlend)
)

type Material struct {
	Name             string
	ImageIdx         *int
	SegmentedTexture *SegmentedTexture
	HorizontalWrap   WrappingMode
	VerticalWrap     WrappingMode
	DoubleSided      bool
	AlphaMode        AlphaMode
}
