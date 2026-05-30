package model

import (
	"bytes"
	"fmt"
	"image/png"
	"sort"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
)

func ExportGLTF(mdl Model, path string) error {
	doc := gltf.NewDocument()
	modelNode := gltf.Node{
		Name:     "Model",
		Children: make([]int, 0),
	}
	doc.Nodes = []*gltf.Node{&modelNode}
	doc.Scenes = []*gltf.Scene{{
		Nodes: []int{0},
	}}
	doc.Scene = new(0)
	var skinIdx *int = nil

	attrs := make([]modeler.PrimitiveAttribute, 0)
	if len(mdl.VertexPositions) > 0 {
		positions := make([][3]float32, len(mdl.VertexPositions))
		for i, p := range mdl.VertexPositions {
			positions[i] = [3]float32(p)
		}
		attrs = append(attrs, modeler.PrimitiveAttribute{
			Name: gltf.POSITION,
			Data: positions,
		})
	}
	if len(mdl.VertexNormals) > 0 {
		normals := make([][3]float32, len(mdl.VertexNormals))
		for i, p := range mdl.VertexNormals {
			normals[i] = [3]float32(p)
		}
		attrs = append(attrs, modeler.PrimitiveAttribute{
			Name: gltf.NORMAL,
			Data: normals,
		})
	}
	if len(mdl.VertexBones) > 0 {
		attrs = append(attrs, modeler.PrimitiveAttribute{
			Name: gltf.JOINTS_0,
			Data: mdl.VertexBones,
		})
	}
	if len(mdl.VertexBoneWeights) > 0 {
		attrs = append(attrs, modeler.PrimitiveAttribute{
			Name: gltf.WEIGHTS_0,
			Data: mdl.VertexBoneWeights,
		})
	}
	if len(mdl.VertexTexCoords_0) > 0 {
		attrs = append(attrs, modeler.PrimitiveAttribute{
			Name: gltf.TEXCOORD_0,
			Data: mgl32Vec2ArrayToGLTF(mdl.VertexTexCoords_0),
		})
	}
	if len(mdl.VertexTexCoords_1) > 0 {
		attrs = append(attrs, modeler.PrimitiveAttribute{
			Name: gltf.TEXCOORD_1,
			Data: mgl32Vec2ArrayToGLTF(mdl.VertexTexCoords_1),
		})
	}
	meshAttrs, err := modeler.WritePrimitiveAttributes(doc, attrs...)
	if err != nil {
		return fmt.Errorf("failed writing primitive accessors: %w", err)
	}

	var boneBaseNodeIdx *int = nil
	if mdl.Skeleton != nil {
		skelNodeIdx := len(doc.Nodes)
		doc.Nodes = append(doc.Nodes, &gltf.Node{Name: "Skeleton", Children: make([]int, 0)})
		doc.Scenes[0].Nodes = append(doc.Scenes[0].Nodes, skelNodeIdx)

		// setup the bone structure and load the inverse bind matrices
		boneBaseNodeIdx = new(len(doc.Nodes))
		numBones := len(mdl.Skeleton.BoneInvBindMatrices)
		joints := make([]int, numBones)
		nodes := make([]*gltf.Node, numBones*2)
		inverseBindMatrices := make([][4][4]float32, numBones)
		for i, m := range mdl.Skeleton.BoneInvBindMatrices {
			// bone node
			nodes[i] = &gltf.Node{Name: mdl.Skeleton.BoneNames[i], Children: make([]int, 0)}
			boneIdx := *boneBaseNodeIdx + i
			// parent node
			nodes[numBones+i] = &gltf.Node{Name: mdl.Skeleton.BoneNames[i] + "_p", Children: []int{boneIdx}}
			prefixBoneIdx := boneIdx + numBones
			t := m.Inv().Col(3).Vec3()
			if mdl.Skeleton.BoneParents[i] > 0 {
				parentBoneIdx := mdl.Skeleton.BoneParents[i]
				parentBindTranslation := mdl.Skeleton.BoneInvBindMatrices[parentBoneIdx].Inv().Col(3).Vec3()
				t = t.Sub(parentBindTranslation)
				nodes[numBones+i].Translation = [3]float64{float64(t[0]), float64(t[1]), float64(t[2])}
				nodes[parentBoneIdx].Children = append(nodes[parentBoneIdx].Children, prefixBoneIdx)
			} else {
				nodes[numBones+i].Translation = [3]float64{float64(t[0]), float64(t[1]), float64(t[2])}
				doc.Nodes[skelNodeIdx].Children = append(doc.Nodes[skelNodeIdx].Children, prefixBoneIdx)
			}

			joints[i] = boneIdx

			inverseBindMatrices[i] = [4][4]float32{
				{m.At(0, 0), m.At(0, 1), m.At(0, 2), m.At(0, 3)},
				{m.At(1, 0), m.At(1, 1), m.At(1, 2), m.At(1, 3)},
				{m.At(2, 0), m.At(2, 1), m.At(2, 2), m.At(2, 3)},
				{m.At(3, 0), m.At(3, 1), m.At(3, 2), m.At(3, 3)},
			}
		}
		doc.Nodes = append(doc.Nodes, nodes...)

		ibmAcc := modeler.WriteInverseBindMatrices(doc, inverseBindMatrices)
		doc.Skins = []*gltf.Skin{{
			Name:                "Skeleton",
			Joints:              joints,
			InverseBindMatrices: &ibmAcc,
			Skeleton:            &skelNodeIdx,
		}}
		skinIdx = new(0)
	}

	meshMapping := make(map[int]int)
	if mdl.Skin != nil {
		// we have a skin so set it up
		doc.Meshes = make([]*gltf.Mesh, 0)
		for meshId, submesh := range mdl.Skin.Meshes {
			if !submesh.IsEquipment {
				indices := make([]uint16, len(submesh.VertexMap))
				for mapIdx, idx := range submesh.VertexMap {
					indices[mapIdx] = uint16(idx)
				}
				idxAcc := modeler.WriteIndices(doc, indices)

				var matIdx *int = nil
				for i, tex := range mdl.Materials {
					if tex.Name == submesh.MaterialName {
						matIdx = &i
					}
				}

				meshIdx := len(doc.Meshes)
				meshMapping[meshId] = meshIdx
				doc.Meshes = append(doc.Meshes, &gltf.Mesh{
					Name: fmt.Sprintf("Mesh%s", submesh.Name),
					Primitives: []*gltf.Primitive{{
						Indices:    gltf.Index(idxAcc),
						Mode:       gltf.PrimitiveTriangles, // todo: get this from the mesh
						Attributes: meshAttrs,
						Material:   matIdx,
					}},
				})
				nodeIdx := len(doc.Nodes)
				doc.Nodes = append(doc.Nodes, &gltf.Node{Name: fmt.Sprintf("Mesh%s", submesh.Name), Mesh: &meshIdx, Skin: skinIdx})
				modelNode.Children = append(modelNode.Children, nodeIdx)
			}
		}
	} else {
		// no skin, render the vertices as points
		indices := make([]uint16, len(mdl.VertexPositions))
		for i, _ := range mdl.VertexPositions {
			indices[i] = uint16(i)
		}
		idxAcc := modeler.WriteIndices(doc, indices)
		doc.Meshes = []*gltf.Mesh{{
			Name: "Mesh",
			Primitives: []*gltf.Primitive{{
				Indices:    gltf.Index(idxAcc),
				Mode:       gltf.PrimitivePoints,
				Attributes: meshAttrs,
			}}},
		}
		nodeIdx := len(doc.Nodes)
		doc.Nodes = append(doc.Nodes, &gltf.Node{Name: "Mesh", Mesh: new(0), Skin: skinIdx})
		modelNode.Children = append(modelNode.Children, nodeIdx)
	}

	// animation export is not correct
	doc.Animations = make([]*gltf.Animation, len(mdl.Animations))
	for i, anim := range mdl.Animations {
		doc.Animations[i] = &gltf.Animation{
			Name:     anim.Name,
			Channels: make([]*gltf.AnimationChannel, 0),
			Samplers: make([]*gltf.AnimationSampler, 0),
		}
		addAnimationTracks(doc, doc.Animations[i], anim.TranslationTracks, *boneBaseNodeIdx, gltf.TRSTranslation)
		addAnimationTracks(doc, doc.Animations[i], anim.RotationTracks, *boneBaseNodeIdx, gltf.TRSRotation)
		addAnimationTracks(doc, doc.Animations[i], anim.ScaleTracks, *boneBaseNodeIdx, gltf.TRSScale)
	}

	doc.Images = make([]*gltf.Image, 0)
	imgBuf := bytes.NewBuffer(make([]byte, 0))
	for i, img := range mdl.Images {
		imgBuf.Reset()
		png.Encode(imgBuf, img)
		if _, err := modeler.WriteImage(doc, fmt.Sprintf("Texture%d", i), "image/png", imgBuf); err != nil {
			panic("failed to write image to the gltf doc")
		}
	}

	defaultTextures := mdl.MakeDefaultTextures()

	doc.Samplers = []*gltf.Sampler{{
		MagFilter: gltf.MagLinear,
		MinFilter: gltf.MinLinearMipMapLinear,
		WrapS:     gltf.WrapClampToEdge,
		WrapT:     gltf.WrapClampToEdge,
	}}
	doc.Textures = make([]*gltf.Texture, len(mdl.Materials))
	doc.Materials = make([]*gltf.Material, len(mdl.Materials))
	for i, mat := range mdl.Materials {
		imgBuf.Reset()
		png.Encode(imgBuf, defaultTextures[i])
		imgIdx := len(doc.Images)
		if _, err := modeler.WriteImage(doc, fmt.Sprintf("DefaultTexture%d", i), "image/png", imgBuf); err != nil {
			panic("failed to write image to the gltf doc")
		}

		doc.Textures[i] = &gltf.Texture{
			Source:  new(imgIdx),
			Sampler: new(0),
		}

		doc.Materials[i] = &gltf.Material{
			Name: mat.Name,
			PBRMetallicRoughness: &gltf.PBRMetallicRoughness{
				BaseColorTexture: &gltf.TextureInfo{Index: i},
				MetallicFactor:   new(0.0),
				RoughnessFactor:  new(1.0),
			},
		}

		if mat.SegmentedTexture != nil {
			doc.Materials[i].Extensions = gltf.Extensions{
				SEGMENTED_TEXTURE_NAME: *mat.SegmentedTexture,
			}
		}
	}

	// we filtered out some of the meshes so we need to remap them to the proper indexes
	elements := make([]ConfigElement, len(mdl.Elements))
	for i, element := range mdl.Elements {
		meshIdxes := make([]int, len(element.MeshIdxes))
		for i, meshIdx := range element.MeshIdxes {
			meshIdxes[i] = meshMapping[meshIdx]
		}
		elements[i] = ConfigElement{
			ChoiceIdxes: element.ChoiceIdxes,
			Materials:   element.Materials,
			MeshIdxes:   meshIdxes,
		}
	}

	doc.Extensions = gltf.Extensions{
		CONFIGURATION_NAME: ConfigExtension{
			Choices:  mdl.Choices,
			Elements: elements,
		},
	}

	return gltf.SaveBinary(doc, path)
}

type trackValue[T any] struct {
	Time  float32
	Value T
}

func addAnimationTracks[T any](doc *gltf.Document, anim *gltf.Animation, tracks []AnimationTrack[T], boneBaseNodeIdx int, path gltf.TRSProperty) {
	for _, track := range tracks {
		if len(track.Timestamps) > 0 {
			if track.Interpolation == InterpolationLinear || track.Interpolation == InterpolationStep {
				trackData := make([]trackValue[T], len(track.Timestamps))
				for i := range trackData {
					trackData[i] = trackValue[T]{Time: track.Timestamps[i], Value: track.Values[i]}
				}
				sort.Slice(trackData, func(i, j int) bool {
					return trackData[i].Time < trackData[j].Time
				})

				timestamps := make([]float32, len(trackData))
				values := make([]T, len(trackData))
				for i, track := range trackData {
					timestamps[i] = track.Time
					values[i] = track.Value
				}

				timeAcc := modeler.WriteAccessor(doc, gltf.TargetArrayBuffer, timestamps)
				transAcc := writeAccessor(doc, gltf.TargetArrayBuffer, values)
				samplerIdx := len(anim.Samplers)
				anim.Samplers = append(anim.Samplers, &gltf.AnimationSampler{
					Interpolation: convertInterpolation(track.Interpolation),
					Input:         timeAcc,
					Output:        transAcc,
				})
				anim.Channels = append(anim.Channels, &gltf.AnimationChannel{
					Sampler: samplerIdx,
					Target: gltf.AnimationChannelTarget{
						Node: new(boneBaseNodeIdx + track.Bone),
						Path: path,
					},
				})
			} else {
				logger.Warn("animation skipped: unsupported interpolation", "animation", anim.Name, "track", path, "bone", track.Bone, "interpolation", track.Interpolation)
			}
		} else if path == gltf.TRSRotation {
			boneName := doc.Nodes[boneBaseNodeIdx+track.Bone].Name
			logger.Debug("empty animation track", "animation", anim.Name, "bone", boneName)
		}
	}
}

func writeAccessor(doc *gltf.Document, target gltf.Target, data any) int {
	if vec3Data, ok := data.([]mgl32.Vec3); ok {
		gltfData := make([][3]float32, len(vec3Data))
		for i, v := range vec3Data {
			gltfData[i] = mgl32Vec3ToGLTF(v)
		}
		return modeler.WriteAccessor(doc, target, gltfData)
	}
	if vec4Data, ok := data.([]mgl32.Vec4); ok {
		gltfData := make([][4]float32, len(vec4Data))
		for i, v := range vec4Data {
			gltfData[i] = mgl32Vec4ToGLTF(v)
		}
		return modeler.WriteAccessor(doc, target, gltfData)
	}
	if mat4Data, ok := data.([]mgl32.Mat4); ok {
		gltfData := make([][4][4]float32, len(mat4Data))
		for i, m := range mat4Data {
			gltfData[i] = mgl32Mat4ToGLTF(m)
		}
		return modeler.WriteAccessor(doc, target, gltfData)
	}
	return modeler.WriteAccessor(doc, target, data)
}

func mgl32Vec2ToGLTF(v mgl32.Vec2) [2]float32 {
	return [2]float32{v.X(), v.Y()}
}

func mgl32Vec2ArrayToGLTF(a []mgl32.Vec2) [][2]float32 {
	gltfA := make([][2]float32, len(a))
	for i := range a {
		gltfA[i] = mgl32Vec2ToGLTF(a[i])
	}
	return gltfA
}

func mgl32Vec3ToGLTF(v mgl32.Vec3) [3]float32 {
	return [3]float32{v.X(), v.Y(), v.Z()}
}

func mgl32Vec4ToGLTF(v mgl32.Vec4) [4]float32 {
	return [4]float32{v.X(), v.Y(), v.Z(), v.W()}
}

func mgl32Mat4ToGLTF(m mgl32.Mat4) [4][4]float32 {
	return [4][4]float32{
		{m.At(0, 0), m.At(0, 1), m.At(0, 2), m.At(0, 3)},
		{m.At(1, 0), m.At(1, 1), m.At(1, 2), m.At(1, 3)},
		{m.At(2, 0), m.At(2, 1), m.At(2, 2), m.At(2, 3)},
		{m.At(3, 0), m.At(3, 1), m.At(3, 2), m.At(3, 3)},
	}
}

func convertInterpolation(interp InterpolationType) gltf.Interpolation {
	switch interp {
	case InterpolationStep:
		return gltf.InterpolationStep
	case InterpolationLinear:
		return gltf.InterpolationLinear
	default:
		panic("unsupported interpolation")
	}
}
