package model

import (
	"fmt"

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
	meshAttrs, err := modeler.WritePrimitiveAttributes(doc, attrs...)
	if err != nil {
		return fmt.Errorf("failed writing primitive accessors: %w", err)
	}

	if mdl.Skeleton != nil {
		skelNodeIdx := len(doc.Nodes)
		doc.Nodes = append(doc.Nodes, &gltf.Node{Name: "Skeleton", Children: make([]int, 0)})
		doc.Scenes[0].Nodes = append(doc.Scenes[0].Nodes, skelNodeIdx)

		// setup the bone structure and load the inverse bind matrices
		boneBaseNodeIdx := len(doc.Nodes)
		joints := make([]int, len(mdl.Skeleton.BoneInvBindMatrices))
		inverseBindMatrices := make([][4][4]float32, len(mdl.Skeleton.BoneInvBindMatrices))
		for i, m := range mdl.Skeleton.BoneInvBindMatrices {
			boneIdx := len(doc.Nodes)
			t := m.Inv().Col(3).Vec3()
			doc.Nodes = append(doc.Nodes, &gltf.Node{Name: fmt.Sprintf("Bone%d", i), Children: make([]int, 0)})
			if mdl.Skeleton.BoneParents[i] > 0 {
				parentBoneIdx := mdl.Skeleton.BoneParents[i]
				parentIdx := boneBaseNodeIdx + parentBoneIdx
				parentBindTranslation := mdl.Skeleton.BoneInvBindMatrices[parentBoneIdx].Inv().Col(3).Vec3()
				t = t.Sub(parentBindTranslation)
				doc.Nodes[boneIdx].Translation = [3]float64{float64(t[0]), float64(t[1]), float64(t[2])}
				doc.Nodes[parentIdx].Children = append(doc.Nodes[parentIdx].Children, boneIdx)
			} else {
				doc.Nodes[boneIdx].Translation = [3]float64{float64(t[0]), float64(t[1]), float64(t[2])}
				doc.Nodes[skelNodeIdx].Children = append(doc.Nodes[skelNodeIdx].Children, boneIdx)
			}

			joints[i] = boneIdx

			inverseBindMatrices[i] = [4][4]float32{
				{m.At(0, 0), m.At(0, 1), m.At(0, 2), m.At(0, 3)},
				{m.At(1, 0), m.At(1, 1), m.At(1, 2), m.At(1, 3)},
				{m.At(2, 0), m.At(2, 1), m.At(2, 2), m.At(2, 3)},
				{m.At(3, 0), m.At(3, 1), m.At(3, 2), m.At(3, 3)},
			}
		}

		ibmAcc := modeler.WriteAccessor(doc, gltf.TargetArrayBuffer, inverseBindMatrices)
		doc.Skins = []*gltf.Skin{{
			Name:                "Skeleton",
			Joints:              joints,
			InverseBindMatrices: &ibmAcc,
			Skeleton:            &skelNodeIdx,
		}}
		skinIdx = new(0)
	}

	if mdl.Skin != nil {
		// we have a skin so set it up
		doc.Meshes = make([]*gltf.Mesh, len(mdl.Skin.Meshes))
		for i, submesh := range mdl.Skin.Meshes {
			indices := make([]uint16, len(submesh.VertexMap))
			for mapIdx, idx := range submesh.VertexMap {
				indices[mapIdx] = uint16(idx)
			}
			idxAcc := modeler.WriteIndices(doc, indices)
			doc.Meshes[i] = &gltf.Mesh{
				Name: fmt.Sprintf("Mesh%s", submesh.Name),
				Primitives: []*gltf.Primitive{{
					Indices:    gltf.Index(idxAcc),
					Mode:       gltf.PrimitiveTriangles, // todo: get this from the mesh
					Attributes: meshAttrs,
				}},
			}
			nodeIdx := len(doc.Nodes)
			doc.Nodes = append(doc.Nodes, &gltf.Node{Name: fmt.Sprintf("Mesh%s", submesh.Name), Mesh: new(i), Skin: skinIdx})
			modelNode.Children = append(modelNode.Children, nodeIdx)
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

	return gltf.Save(doc, path)
}
