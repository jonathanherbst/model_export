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
		gltf_path, err := cmd.Flags().GetString("gltf")
		if err != nil {
			panic("no gltf flag")
		}

		if chunks {
			m2Reader := blizzard.M2Reader{Reader: m2File}
			for header, data := range m2Reader.Chunks {
				fmt.Printf("Chunk %s - %d bytes\n", string(header.Token[:]), len(data))
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
					fmt.Fprintf(os.Stderr, "failed to parse m2 file: %v\n", err)
					os.Exit(1)
				}
			}

			if gltf_path != "" {
				export_gltf(m2, skin, gltf_path)
				return
			}

			fmt.Printf("M2 file has %d vertices\n", len(m2.Vertices))
			fmt.Printf("Skin file ids: %v\n", m2.SkinFileIds)
			for _, vertex := range m2.Vertices[:10] {
				fmt.Printf("\t%v\n", vertex)
			}
			if skin != nil {
				fmt.Printf("Skin: %d meshes\n", len(skin.Meshes))
				for _, submesh := range skin.Meshes {
					fmt.Printf("\tid: %d, %d verticies\n", submesh.Id, len(submesh.LocalVertexIdxes))
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
}

func export_gltf(m2 *blizzard.M2, skin *blizzard.M2Skin, gltf_path string) {
	doc := gltf.NewDocument()

	if skin != nil {
		// skin uses a subset of vertices from the m2 vertex list
		positions := make([][3]float32, len(skin.VertexIdxes))
		normals := make([][3]float32, len(skin.VertexIdxes))
		for i, vi := range skin.VertexIdxes {
			v := m2.Vertices[vi]
			positions[i] = [3]float32{v.Pos.X, v.Pos.Z, -v.Pos.Y}
			normals[i] = [3]float32{v.Normal.X, v.Normal.Z, -v.Normal.Y}
		}
		attrs, _ := modeler.WritePrimitiveAttributes(doc,
			modeler.PrimitiveAttribute{Name: gltf.POSITION, Data: positions},
			modeler.PrimitiveAttribute{Name: gltf.NORMAL, Data: normals},
		)

		renderMeshes := skin.Meshes
		doc.Meshes = make([]*gltf.Mesh, len(renderMeshes))
		doc.Nodes = make([]*gltf.Node, len(renderMeshes))
		doc.Scenes[0].Nodes = make([]int, len(renderMeshes))
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
			doc.Nodes[i] = &gltf.Node{Name: fmt.Sprintf("Model%d", submesh.Id), Mesh: new(i)}
			doc.Scenes[0].Nodes[i] = i
			doc.Scene = new(0)
		}
	} else {
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
