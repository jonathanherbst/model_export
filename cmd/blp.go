package cmd

import (
	"fmt"
	"image/png"
	"jph/model-export/pkg/blizzard"
	"os"

	"github.com/spf13/cobra"
)

// blpCmd represents the blp command
var blpCmd = &cobra.Command{
	Use:   "blp [flags] blp_path",
	Args:  cobra.ExactArgs(1),
	Short: "Convert blp files",
	Long:  `Convert blp files to pngs`,
	Run: func(cmd *cobra.Command, args []string) {
		png_path, err := cmd.Flags().GetString("png")
		if err != nil {
			panic("wtf")
		}

		blp, err := blizzard.BLPFromFile(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(2)
		}

		if png_path != "" {
			f, err := os.Create(png_path)
			if err == nil {
				img, err := blp.Decode(0)
				if err != nil {
					fmt.Fprintf(os.Stderr, "failed to decode: %v\n", err)
				}
				png.Encode(f, img)
				f.Close()
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(blpCmd)
	blpCmd.Flags().String("png", "", "Export to a png file")
}
