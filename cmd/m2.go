/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"jph/model-export/pkg/blizzard"
	"os"

	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
	"github.com/spf13/cobra"
)

// m2Cmd represents the m2 command
var m2Cmd = &cobra.Command{
	Use:   "m2 [flags] <m2_path>",
	Args:  cobra.ExactArgs(1),
	Short: "Parse an m2 file",
	Long:  `Parse an m2 file and show lots of information about it or even export the verticies and animations to a gltf file`,
	Run: func(cmd *cobra.Command, args []string) {
		m2File, err := os.Open(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open m2 file: %v\n", err)
			os.Exit(1)
		}

		chunks, err := cmd.Flags().GetBool("chunks")
		if err != nil {
			panic("no chunks flag")
		}
		skin_path, err := cmd.Flags().GetString("skin")
		if err != nil {
			panic("no skin flag")
		}
		skel_path, err := cmd.Flags().GetString("skel")
		if err != nil {
			panic("no skel flag")
		}
		gltf_path, err := cmd.Flags().GetString("gltf")
		if err != nil {
			panic("no gltf flag")
		}

		if chunks {
			m2Reader := blizzard.M2ChunkReader{Reader: m2File}
			for header, data := range m2Reader.Chunks {
				fmt.Printf("Chunk %s - %d bytes\n", string(header.Token[:]), len(data))
			}
			if skel_path != "" {
				skelFile, err := os.Open(skel_path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to open skel file: %v\n", err)
					os.Exit(1)
				}

				fmt.Printf("Skel file: %s\n", skel_path)
				skelReader := blizzard.M2ChunkReader{Reader: skelFile}
				for header, data := range skelReader.Chunks {
					fmt.Printf("Chunk %s - %d bytes\n", string(header.Token[:]), len(data))
				}
			}
		} else {
			m2, err := blizzard.M2FromReader(m2File)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to parse m2 file: %v\n", err)
				os.Exit(1)
			}

			var skin *blizzard.M2Skin
			if skin_path != "" {
				skin, err = blizzard.M2SkinFromFile(skin_path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to parse skin file: %v\n", err)
					os.Exit(1)
				}
			}

			var skel *blizzard.M2Skeleton
			if skel_path != "" {
				skel, err = blizzard.M2SkelFromFile(skel_path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to parse skel file: %v\n", err)
					os.Exit(1)
				}
			}

			if gltf_path != "" {
				export_gltf(m2, skin, skel, gltf_path)
				return
			}

			fmt.Printf("M2 file has %d vertices, %d bones, %d sequences\n", len(m2.Vertices), len(m2.Bones), len(m2.Sequences))
			fmt.Printf("Skin file ids: %v\n", m2.SkinFileIds)
			fmt.Printf("Skel file ids: %v\n", m2.SkelFileIds)
			for _, vertex := range m2.Vertices[:10] {
				fmt.Printf("\t%v\n", vertex)
			}
			if skin != nil {
				fmt.Printf("Skin: %d meshes\n", len(skin.Meshes))
				for _, submesh := range skin.Meshes {
					fmt.Printf("\tid: %d, %d verticies\n", submesh.Id, len(submesh.LocalVertexIdxes))
				}
			}
			if skel != nil {
				fmt.Printf("Skel: %s, %d bones, %d sequences", skel.Name, len(skel.Bones), len(skel.Sequences))
				if skel.ParentSkelFileId != nil {
					fmt.Printf(", parent %d\n", *skel.ParentSkelFileId)
				} else {
					fmt.Println()
				}
				fmt.Printf("\tSeq Ids: %v\n", skel.SequenceIds)
				fmt.Printf("\tSeq Ids: [")
				has808 := false
				for i, seq := range skel.Sequences {
					if seq.ID == 808 {
						has808 = true
					}
					if i == 0 {
						fmt.Printf("%d", seq.ID)
					} else {
						fmt.Printf(" %d", seq.ID)
					}
				}
				fmt.Println("]")
				fmt.Printf("\tHas Seq 808: %v\n", has808)
				fmt.Printf("\tAnims: %v\n", skel.AnimMeta)
				fmt.Printf("\tBoneFiles: %v\n", skel.BoneFileIds)
				fmt.Println("Bones:")
				for _, bone := range skel.Bones {
					fmt.Printf("\tid: %d, parent: %d, flags: %08X, pivot: %v\n", bone.KeyBoneId, bone.ParentBone, bone.Flags, bone.Pivot)
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(m2Cmd)
	m2Cmd.Flags().Bool("chunks", false, "Print out the chunks types and sizes from m2 file")
	m2Cmd.Flags().String("gltf", "", "Export the m2 file to a gltf")
	m2Cmd.Flags().String("skin", "", "Skin file containing the geosets of the m2 file")
	m2Cmd.Flags().String("skel", "", "Skel file containing the bone structure and animations for the bones")
}

func export_gltf(m2 *blizzard.M2, skin *blizzard.M2Skin, skel *blizzard.M2Skeleton, gltf_path string) {
	doc := gltf.NewDocument()

	if skin != nil {
		// skin uses a subset of vertices from the m2 vertex list
		positions := make([][3]float32, len(skin.VertexIdxes))
		normals := make([][3]float32, len(skin.VertexIdxes))
		joints := make([][4]uint8, len(skin.VertexIdxes))
		weights := make([][4]uint8, len(skin.VertexIdxes))
		for i, vi := range skin.VertexIdxes {
			v := m2.Vertices[vi]
			positions[i] = [3]float32{v.Pos.X, v.Pos.Z, -v.Pos.Y}
			normals[i] = [3]float32{v.Normal.X, v.Normal.Z, -v.Normal.Y}
			joints[i] = v.BoneIndices
			weights[i] = v.BoneWeights
		}
		attrs, _ := modeler.WritePrimitiveAttributes(doc,
			modeler.PrimitiveAttribute{Name: gltf.POSITION, Data: positions},
			modeler.PrimitiveAttribute{Name: gltf.NORMAL, Data: normals},
			modeler.PrimitiveAttribute{Name: gltf.JOINTS_0, Data: joints},
			modeler.PrimitiveAttribute{Name: gltf.WEIGHTS_0, Data: weights},
		)

		renderMeshes := skin.Meshes
		doc.Meshes = make([]*gltf.Mesh, len(renderMeshes))
		doc.Nodes = make([]*gltf.Node, len(renderMeshes)+1)
		doc.Scenes[0].Nodes = make([]int, 0)
		modelNode := gltf.Node{
			Name:     "Model",
			Children: make([]int, len(renderMeshes)),
		}
		doc.Nodes[0] = &modelNode
		doc.Scenes[0].Nodes = append(doc.Scenes[0].Nodes, 0)

		// build the meshes
		doc.Meshes = make([]*gltf.Mesh, len(renderMeshes))
		meshNodes := doc.Nodes[1:]
		for i, submesh := range renderMeshes {
			for _, idx := range submesh.LocalVertexIdxes {
				if idx >= uint16(len(skin.VertexIdxes)) {
					fmt.Printf("ERROR: LocalVertexIdx %d >= len(skin.VertexIdxes) %d\n", idx, len(skin.VertexIdxes))
				}
			}
			idxAcc := modeler.WriteIndices(doc, submesh.LocalVertexIdxes)
			doc.Meshes[i] = &gltf.Mesh{
				Name: fmt.Sprintf("Mesh%d", submesh.Id),
				Primitives: []*gltf.Primitive{{
					Indices:    gltf.Index(idxAcc),
					Mode:       gltf.PrimitiveTriangles,
					Attributes: attrs,
				}},
			}
			nodeIdx := i + 1
			doc.Nodes[nodeIdx] = &gltf.Node{Name: fmt.Sprintf("Mesh%d", submesh.Id), Mesh: new(i)}
			modelNode.Children[i] = nodeIdx
		}

		if skel != nil {
			// build the skeleton
			skeletonNode := &gltf.Node{
				Name:     "Skeleton",
				Children: make([]int, 0),
			}
			skeletonNodeId := len(doc.Nodes)
			doc.Nodes = append(doc.Nodes, skeletonNode)
			doc.Scenes[0].Nodes = append(doc.Scenes[0].Nodes, skeletonNodeId)

			inverseBindMatrices := make([][4][4]float32, len(skel.Bones))
			joints := make([]int, len(skel.Bones))
			boneLookup := make(map[int32]int)
			for i, bone := range skel.Bones {
				joints[i] = len(doc.Nodes)
				if _, ok := boneLookup[bone.KeyBoneId]; ok {
					panic("two bones have the same key id")
				}
				if bone.KeyBoneId >= 0 {
					boneLookup[bone.KeyBoneId] = len(doc.Nodes)
				}

				translation := [3]float64{float64(bone.Pivot.X), float64(bone.Pivot.Z), float64(-bone.Pivot.Y)}
				inverseBindMatrices[i] = [4][4]float32{
					{1, 0, 0, -bone.Pivot.X},
					{0, 1, 0, -bone.Pivot.Z},
					{0, 0, 1, bone.Pivot.Y},
					{0, 0, 0, 1},
				}

				if bone.ParentBone == -1 {
					skeletonNode.Children = append(skeletonNode.Children, joints[i])
				} else {
					parentNode := doc.Nodes[joints[bone.ParentBone]]
					parentBone := skel.Bones[bone.ParentBone]
					translation[0] -= float64(parentBone.Pivot.X)
					translation[1] -= float64(parentBone.Pivot.Z)
					translation[2] += float64(parentBone.Pivot.Y)
					parentNode.Children = append(parentNode.Children, joints[i])
				}

				doc.Nodes = append(doc.Nodes, &gltf.Node{
					Name:        fmt.Sprintf("Bone%d", i),
					Translation: translation,
					Children:    make([]int, 0),
				})
			}

			ibmAcc := modeler.WriteAccessor(doc, gltf.TargetArrayBuffer, inverseBindMatrices)
			doc.Skins = []*gltf.Skin{{
				Name:                "Skeleton",
				Joints:              joints,
				InverseBindMatrices: &ibmAcc,
				Skeleton:            &skeletonNodeId,
			}}
			for _, mesh := range meshNodes {
				mesh.Skin = new(0)
			}
		}
		doc.Scene = new(0)
	} else {
		// write the vertices as points if there's no skin
		positions := make([][3]float32, len(m2.Vertices))
		normals := make([][3]float32, len(m2.Vertices))
		for i, v := range m2.Vertices {
			positions[i] = [3]float32{v.Pos.X, v.Pos.Z, -v.Pos.Y}
			normals[i] = [3]float32{v.Normal.X, v.Normal.Z, -v.Normal.Y}
		}
		attrs, _ := modeler.WritePrimitiveAttributes(doc,
			modeler.PrimitiveAttribute{Name: gltf.POSITION, Data: positions},
			modeler.PrimitiveAttribute{Name: gltf.NORMAL, Data: normals},
		)
		indices := make([]uint32, len(positions))
		for i := range indices {
			indices[i] = uint32(i)
		}
		idxAcc := modeler.WriteIndices(doc, indices)
		doc.Meshes = []*gltf.Mesh{{
			Name: "Mesh",
			Primitives: []*gltf.Primitive{{
				Indices:    &idxAcc,
				Mode:       gltf.PrimitivePoints,
				Attributes: attrs,
			}},
		}}
		doc.Nodes = []*gltf.Node{{Name: "Model", Mesh: new(0)}}
		doc.Scenes[0].Nodes = []int{0}
		doc.Scene = new(0)
	}

	if err := gltf.Save(doc, gltf_path); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save gltf: %v\n", err)
		os.Exit(1)
	}
}
