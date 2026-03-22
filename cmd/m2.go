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
			panic("no chunks flag?")
		}
		gltf_path, err := cmd.Flags().GetString("gltf")
		if err != nil {
			panic("no gltf flag?")
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

			if gltf_path != "" {
				doc := gltf.NewDocument()
				doc.Meshes = []*gltf.Mesh{{
					Name: "Model",
					Primitives: []*gltf.Primitive{{
						Indices: gltf.Index(modeler.WriteIndices(doc, []uint16{0, 1, 2})),
						Attributes: gltf.PrimitiveAttributes{
							gltf.POSITION: modeler.WritePosition(doc, [][3]float32{{0, 0, 0}, {0, 10, 0}, {0, 0, 10}}),
							gltf.COLOR_0:  modeler.WriteColor(doc, [][3]uint8{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}}),
						},
					}},
				}}
				doc.Nodes = []*gltf.Node{{Name: "Model", Mesh: gltf.Index(0)}}
				doc.Scenes[0].Nodes = append(doc.Scenes[0].Nodes, 0)
				gltf.Save(doc, gltf_path)
			}

			fmt.Printf("M2 file has %d vertices\n", len(m2.Vertices))
			for _, vertex := range m2.Vertices[:10] {
				fmt.Printf("\t%v\n", vertex)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(m2Cmd)
	m2Cmd.Flags().Bool("chunks", false, "Print out the chunks types and sizes from m2 file")
	m2Cmd.Flags().String("gltf", "", "Export the m2 file to a gltf")
}
