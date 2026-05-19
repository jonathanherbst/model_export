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
	"path/filepath"
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

		mat_path, err := cmd.Flags().GetString("mat")
		if err != nil {
			panic("no mat flag")
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

				if modelName != nil {
					mdl := wowLoadModel(wow, *modelName, gender)
					if glb_path != "" {
						model.ExportGLTF(mdl, glb_path)
					} else if mat_path != "" {
						wowExportMat(mdl, mat_path)
					}
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

func wowLoadModel(wow *blizzard.WOWCasc, modelName string, gender *int) model.Model {
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

	return *mdl
}

func wowExportTex(mdl model.Model, path string) {
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

	os.Mkdir(path, 0750)
	for i, component := range mdl.Configurations {
		selected := true
		optionString := ""
		for _, choice := range component.Configurations {
			selected = selected && selectedOptions[choice.OptionName].OrderIndex == choice.OrderIndex
			optionString += fmt.Sprintf("%s(%s-%08X-%d)", choice.OptionName, choice.Name, choice.Color, choice.OrderIndex)
		}
		if selected && len(component.TextureFragments) > 0 {
			for _, frag := range component.TextureFragments {
				texName := filepath.Join(path, fmt.Sprintf("%d-%s.png", i, optionString))
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
}

func wowExportMat(mdl model.Model, path string) {
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

	os.Mkdir(path, 0750)
	selectedComponents := make([]model.ConfigurationComponent, 0)
	for _, component := range mdl.Configurations {
		selected := true
		for _, choice := range component.Configurations {
			selected = selected && selectedOptions[choice.OptionName].OrderIndex == choice.OrderIndex
		}
		if selected && len(component.TextureFragments) > 0 {
			selectedComponents = append(selectedComponents, component)
		}
	}

	textureFrags := make([]model.TextureFragment, 0)
	for _, component := range selectedComponents {
		for _, frag := range component.TextureFragments {
			textureFrags = append(textureFrags, frag)
		}
	}
	sort.Slice(textureFrags, func(i, j int) bool { return textureFrags[i].Layer < textureFrags[j].Layer })

	for matIdx, mat := range mdl.Materials {
		texture := image.NewRGBA(image.Rect(0, 0, int(mat.Width), int(mat.Height)))
		for _, frag := range textureFrags {
			if frag.MaterialIdx == matIdx {
				texImg := mdl.Images[frag.Img]
				// need to resize the image to the fragment size
				img := image.NewRGBA(image.Rect(0, 0, int(frag.Width), int(frag.Height)))
				draw.BiLinear.Scale(img, img.Rect, texImg, texImg.Bounds(), draw.Over, nil)
				rect := img.Bounds().Add(image.Point{int(frag.X), int(frag.Y)})
				draw.Draw(texture, rect, img, image.Point{0, 0}, draw.Over)
			}
		}

		matPath := filepath.Join(path, fmt.Sprintf("mat-%d.png", matIdx))
		matFile, err := os.Create(matPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open mat file: %v\n", err)
			os.Exit(2)
		}
		png.Encode(matFile, texture)
		matFile.Close()

	}

}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().String("casc", "", "Path to a casc")
	exportCmd.Flags().String("glb", "", "Path to export the glb of the specified model")
	exportCmd.Flags().String("mat", "", "Directory to export default materials")
}
