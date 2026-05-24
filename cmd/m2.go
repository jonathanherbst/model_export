/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"jph/model-export/pkg/blizzard"
	"jph/model-export/pkg/model"
	"os"

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
				exportGLTF(m2, skin, skel, gltf_path)
				return
			}

			fmt.Printf("M2 file has %d vertices, %d bones, %d sequences\n", len(m2.Vertices), len(m2.Bones), len(m2.Sequences))
			fmt.Printf("Skin file ids: %v\n", m2.SkinFileIds)
			fmt.Printf("Skel file ids: %v\n", m2.SkelFileIds)
			fmt.Printf("Texture file ids: %v\n", m2.TextureFileIds)
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
				var first808Idx *int = nil
				for i, seq := range skel.Sequences {
					if seq.ID == 0 && first808Idx == nil {
						first808Idx = &i
					}
					if i == 0 {
						fmt.Printf("%d", seq.ID)
					} else {
						fmt.Printf(" %d", seq.ID)
					}
				}
				fmt.Println("]")
				fmt.Printf("\tHas Seq 808: %d\n", first808Idx)
				fmt.Printf("\tAnims: %v\n", skel.AnimMeta)
				fmt.Printf("\tBoneFiles: %v\n", skel.BoneFileIds)
				fmt.Println("Bones:")
				for _, bone := range skel.Bones {
					animLen := 0
					if first808Idx != nil && len(bone.Rotation.Timestamps) > *first808Idx {
						animLen = len(bone.Rotation.Timestamps[*first808Idx])
					}
					fmt.Printf("\tid: %d, 808 rot: %d\n", bone.KeyBoneId, animLen)
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

func exportGLTF(m2 *blizzard.M2, skin *blizzard.M2Skin, skel *blizzard.M2Skeleton, gltf_path string) {
	// fill up a model with the blizzard parameters
	var mdl model.Model
	m2.FillModel(&mdl, nil)
	skin.FillModel(&mdl, *m2)
	skel.FillModel(&mdl)

	// export the model
	if err := model.ExportGLTF(mdl, gltf_path); err != nil {
		fmt.Fprintf(os.Stderr, "failed to save gltf: %v\n", err)
		os.Exit(1)
	}
}
