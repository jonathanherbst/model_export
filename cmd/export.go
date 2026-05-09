/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"io"
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
	var raceId int = -1
	if races, err := wow.GetTable("ChrRaces"); err == nil {
		for record := range races.GetRecords {
			if record.GetStringFieldByName("Name_lang") == modelName {
				raceId = int(record.GetID())
				break
			}
		}
	}

	if raceId < 0 {
		fmt.Println("Couldn't find the model")
		os.Exit(1)
	}

	var modelId int = -1
	if chrxmodels, err := wow.GetTable("ChrRaceXChrModel"); err == nil {
		for record := range func(yield func(blizzard.DBDRecord) bool) {
			chrxmodels.GetFixedRecordsByForeignKey(uint32(raceId), yield)
		} {
			if gender != nil && int(record.GetIntFieldByName("Sex")) == *gender {
				modelId = int(record.GetIntFieldByName("ChrModelID"))
				break
			}
		}
	}

	if modelId < 0 {
		fmt.Println("Couldn't find the model")
		os.Exit(1)
	}

	var displayId int = -1
	if chrmodels, err := wow.GetTable("ChrModel"); err == nil {
		if record := chrmodels.GetFixedRecordById(uint32(modelId)); record != nil {
			displayId = int(record.GetIntFieldByName("DisplayID"))
		}
	}
	if displayId < 0 {
		fmt.Println("Failed to find ChrModel")
		os.Exit(1)
	}

	var modelDataId int = -1
	if chrDisplayInfo, err := wow.GetTable("CreatureDisplayInfo"); err == nil {
		if record := chrDisplayInfo.GetFixedRecordById(uint32(displayId)); record != nil {
			modelDataId = int(record.GetIntFieldByName("ModelID"))
		}
	}
	if modelDataId < 0 {
		fmt.Println("Failed to find CreatureDisplayInfo")
		os.Exit(1)
	}

	var fileDataId int = -1
	if creatureModelData, err := wow.GetTable("CreatureModelData"); err == nil {
		if record := creatureModelData.GetFixedRecordById(uint32(modelDataId)); record != nil {
			fileDataId = int(record.GetIntFieldByName("FileDataID"))
		}
	}
	if fileDataId < 0 {
		fmt.Println("Failed to find model file id")
		os.Exit(1)
	}

	fmt.Printf("model file id: %d\n", fileDataId)

	var mdl model.Model

	modelFile, err := wow.Casc.OpenFileById(uint32(fileDataId), false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open model file: %v\n", err)
		os.Exit(1)
	}

	m2File, err := blizzard.M2FromReader(modelFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse model file: %v\n", err)
		os.Exit(0)
	}
	m2File.FillModel(&mdl)

	if len(m2File.SkinFileIds) > 0 {
		skinFile, err := wow.Casc.OpenFileById(uint32(m2File.SkinFileIds[0]), false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open skin file: %v\n", err)
			os.Exit(1)
		}
		buf, err := io.ReadAll(skinFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read skin file: %v\n", err)
			os.Exit(1)
		}
		skin, err := blizzard.M2SkinFromBuf(buf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse skin file: %v\n", err)
			os.Exit(1)
		}
		skin.FillModel(&mdl)
	}

	for _, skelFileId := range m2File.SkelFileIds {
		skelFile, err := wow.Casc.OpenFileById(uint32(skelFileId), false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open skel file: %v\n", err)
			os.Exit(1)
		}
		skel, err := blizzard.M2SkelFromReader(skelFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse skel file: %v\n", err)
			os.Exit(1)
		}
		skel.FillModel(&mdl)
	}

	model.ExportGLTF(mdl, path)
}

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().String("casc", "", "Path to a casc")
	exportCmd.Flags().String("glb", "", "Path to export the glb of the specified model")
}
