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

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().String("casc", "", "Path to a casc")
	exportCmd.Flags().String("glb", "", "Path to export the glb of the specified model")
}
