/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"image/png"
	"jph/model-export/pkg/blizzard"
	"jph/model-export/pkg/model"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
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
	selectedOptions := make(map[string]int)
	for i, choice := range mdl.Choices {
		if _, ok := selectedOptions[choice.Option]; !ok {
			selectedOptions[choice.Option] = i
		}
	}

	os.Mkdir(path, 0750)
	for _, element := range mdl.Elements {
		selected := true
		optionString := ""
		for _, choiceIdx := range element.ChoiceIdxes {
			choice := mdl.Choices[choiceIdx]
			selected = selected && (selectedOptions[choice.Option] == choiceIdx)
			optionString += fmt.Sprintf("%s(%s-%08X)", choice.Option, choice.Choice, choice.Color)
		}

		if selected && len(element.Materials) > 0 {
			for _, mat := range element.Materials {
				texName := filepath.Join(path, fmt.Sprintf("%s.png", optionString))
				file, err := os.Create(texName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to open texture file: %v\n", err)
					os.Exit(2)
				}
				png.Encode(file, mdl.Images[mat.ImageIdx])
				file.Close()
			}
		}
	}
}

func wowExportMat(mdl model.Model, path string) {
	os.Mkdir(path, 0750)
	defaultTextures := mdl.MakeDefaultTextures()
	for matIdx, mat := range mdl.Materials {
		matPath := filepath.Join(path, fmt.Sprintf("%s.png", mat.Name))
		matFile, err := os.Create(matPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open mat file: %v\n", err)
			os.Exit(2)
		}
		png.Encode(matFile, defaultTextures[matIdx])
		matFile.Close()

	}

}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().String("casc", "", "Path to a casc")
	exportCmd.Flags().String("glb", "", "Path to export the glb of the specified model")
	exportCmd.Flags().String("mat", "", "Directory to export default materials")
}
