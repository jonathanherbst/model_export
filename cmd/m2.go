/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"jph/model-export/pkg/blizzard"
	"math"
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
					fmt.Printf("\tid: %d, %d verticies\n", submesh.Id, len(submesh.VertexIndexes))
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

	vertexCount := len(m2.Vertices)
	positions := make([][3]float32, vertexCount)
	normals := make([][3]float32, vertexCount)
	for i, v := range m2.Vertices {
		// M2 is Z-up; convert to glTF Y-up via (x, z, -y)
		positions[i] = [3]float32{v.Pos.X, -v.Pos.Z, v.Pos.Y}
		normals[i] = [3]float32{v.Normal.X, -v.Normal.Z, v.Normal.Y}
		normalMag := math.Sqrt(float64(v.Normal.X*v.Normal.X + v.Normal.Z*v.Normal.Z + v.Normal.Y*v.Normal.Y))
		if normalMag == 0 || math.IsNaN(normalMag) || math.IsInf(normalMag, 0) {
			// Default to Y-up unit normal if zero
			normals[i] = [3]float32{0, 1, 0}
		} else {
			normals[i][0] = normals[i][0] / float32(normalMag)
			normals[i][1] = normals[i][1] / float32(normalMag)
			normals[i][2] = normals[i][2] / float32(normalMag)
		}
	}

	fmt.Printf("vertex count: %d\n", vertexCount)

	attrs, _ := modeler.WritePrimitiveAttributes(doc,
		modeler.PrimitiveAttribute{Name: gltf.POSITION, Data: positions},
		modeler.PrimitiveAttribute{Name: gltf.NORMAL, Data: normals},
	)

	// posAcc := modeler.WritePosition(doc, positions)
	// nrmAcc := modeler.WriteNormal(doc, normals)

	indexCount := 0

	fmt.Printf("idx: %v\n", skin.Meshes[0].VertexIndexes[:10])

	primitives := make([]*gltf.Primitive, 1)
	for i, submesh := range skin.Meshes[:1] {
		indexCount += len(submesh.VertexIndexes)
		idxAcc := modeler.WriteIndices(doc, submesh.VertexIndexes)
		primitives[i] = &gltf.Primitive{
			Indices:    gltf.Index(idxAcc),
			Mode:       gltf.PrimitiveTriangles,
			Attributes: attrs,
			Material:   new(0),
		}
	}
	fmt.Printf("index count: %d\n", indexCount)

	doc.Meshes = []*gltf.Mesh{{
		Name:       "Model",
		Primitives: primitives,
	}}

	doc.Materials = []*gltf.Material{{PBRMetallicRoughness: &gltf.PBRMetallicRoughness{
		BaseColorFactor: &[4]float64{1.000, 0.766, 0.336, 1.0},
		MetallicFactor:  new(0.5),
		RoughnessFactor: new(0.1),
	}}}

	doc.Nodes = []*gltf.Node{{Name: "Model", Mesh: gltf.Index(0)}}
	doc.Scenes[0].Nodes = append(doc.Scenes[0].Nodes, 0)

	if err := gltf.Save(doc, gltf_path); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save gltf: %v\n", err)
		os.Exit(1)
	}
}
