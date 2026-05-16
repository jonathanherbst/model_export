/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"image"
	"image/png"
	"jph/model-export/pkg/blizzard"
	"jph/model-export/pkg/model"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"golang.org/x/image/draw"
)

// exportCmd represents the exportchar command
var exportCmd = &cobra.Command{
	Use:   "export [flags] [model_name 0|1|2|m|f]",
	Short: "Export models from from a game to a glb file",
	Long:  `Export export any models from a game to a glb file`,
	Run: func(cmd *cobra.Command, args []string) {
		casc_path, err := cmd.Flags().GetString("casc")
		if err != nil {
			panic("no casc flag")
		}

		glb_path, err := cmd.Flags().GetString("glb")
		if err != nil {
			panic("no glb flag")
		}

		tex_path, err := cmd.Flags().GetString("tex")
		if err != nil {
			panic("no tex flag")
		}

		var modelName *string = nil
		if len(args) > 0 {
			modelName = &args[0]
		}

		var gender *int = nil
		if len(args) > 1 {
			switch args[1] {
			case "m":
				fallthrough
			case "0":
				gender = new(0)
			case "f":
				fallthrough
			case "1":
				gender = new(1)
			case "2":
				gender = new(2)
			}
		}

		if casc_path != "" {
			casc, err := blizzard.OpenCasc(casc_path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open casc: %v", err)
				os.Exit(1)
			}

			if blizzard.IsWOWCasc(casc) {
				wow, err := blizzard.OpenWOWCasc(casc, ".")
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to open wow casc: %v", err)
					os.Exit(1)
				}

				if glb_path != "" && modelName != nil {
					wowExportModel(wow, glb_path, *modelName, gender)
				} else if tex_path != "" && modelName != nil {
					wowExportTex(wow, tex_path, *modelName, gender)
				} else {
					wowPrintRaces(wow)
				}
			}
		}
	},
}

func wowPrintRaces(wow *blizzard.WOWCasc) {
	if races, err := wow.GetTable("ChrRaces"); err == nil {
		for record := range races.GetRecords {
			name := record.GetStringFieldByName("Name_lang")
			loreName := record.GetStringFieldByName("Lore_name_lang")
			fmt.Printf("%s (%s)\n", name, loreName)
		}
	}
}

func wowExportModel(wow *blizzard.WOWCasc, path string, modelName string, gender *int) {
	modelId := wow.FindRaceModelId(modelName, gender)
	if modelId == nil {
		fmt.Fprintln(os.Stderr, "Failed to find the model id")
		os.Exit(1)
	}

	mdl := wow.LoadModelFromId(*modelId)
	if mdl == nil {
		fmt.Fprintln(os.Stderr, "Failed to load the model")
		os.Exit(1)
	}

	model.ExportGLTF(*mdl, path)
}

func wowExportTex(wow *blizzard.WOWCasc, path string, modelName string, gender *int) {
	modelId := wow.FindRaceModelId(modelName, gender)
	if modelId == nil {
		fmt.Fprintln(os.Stderr, "Failed to find the model id")
		os.Exit(1)
	}

	mdl := wow.LoadModelFromId(*modelId)
	if mdl == nil {
		fmt.Fprintln(os.Stderr, "Failed to load the model")
		os.Exit(1)
	}

	selectedOptions := make(map[string]model.ConfigurationChoice)
	for _, component := range mdl.Configurations {
		for _, choice := range component.Configurations {
			if currentChoice, ok := selectedOptions[choice.OptionName]; ok {
				if choice.OrderIndex < currentChoice.OrderIndex {
					selectedOptions[choice.OptionName] = choice
				}
			} else {
				selectedOptions[choice.OptionName] = choice
			}
		}
	}

	os.RemoveAll("texture")
	os.Mkdir("texture", 0750)
	selectedComponents := make([]model.ConfigurationComponent, 0)
	for i, component := range mdl.Configurations {
		selected := true
		optionString := ""
		for _, choice := range component.Configurations {
			selected = selected && selectedOptions[choice.OptionName].OrderIndex == choice.OrderIndex
			optionString += fmt.Sprintf("%s(%s-%08X-%d)", choice.OptionName, choice.Name, choice.Color, choice.OrderIndex)
		}
		if selected && len(component.TextureFragments) > 0 {
			selectedComponents = append(selectedComponents, component)

			for _, frag := range component.TextureFragments {
				texName := fmt.Sprintf("texture/%d-%s.png", i, optionString)
				file, err := os.Create(texName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to open texture file: %v\n", err)
					os.Exit(2)
				}
				png.Encode(file, mdl.Images[frag.Img])
				file.Close()
			}
		}
	}

	textureFrags := make([]model.TextureFragment, 0)
	for _, component := range selectedComponents {
		for _, frag := range component.TextureFragments {
			textureFrags = append(textureFrags, frag)
		}
	}
	sort.Slice(textureFrags, func(i, j int) bool { return textureFrags[i].Layer < textureFrags[j].Layer })

	texture := image.NewRGBA(image.Rect(0, 0, int(mdl.Textures[0].Width), int(mdl.Textures[0].Height)))
	for _, frag := range textureFrags {
		texImg := mdl.Images[frag.Img]
		// need to resize the image to the fragment size
		img := image.NewRGBA(image.Rect(0, 0, int(frag.Width), int(frag.Height)))
		draw.BiLinear.Scale(img, img.Rect, texImg, texImg.Bounds(), draw.Over, nil)
		rect := img.Bounds().Add(image.Point{int(frag.X), int(frag.Y)})
		draw.Draw(texture, rect, img, image.Point{0, 0}, draw.Over)
	}
	texFile, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open tex file: %v\n", err)
		os.Exit(2)
	}
	png.Encode(texFile, texture)
	texFile.Close()
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().String("casc", "", "Path to a casc")
	exportCmd.Flags().String("glb", "", "Path to export the glb of the specified model")
	exportCmd.Flags().String("tex", "", "Export the default combined texture")
}
