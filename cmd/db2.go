/*
Copyright © 2026 Jonathan Herbst <amd64d@gmail.com>
*/
package cmd

import (
	"fmt"
	"jph/model-export/pkg/blizzard"
	"os"

	"github.com/spf13/cobra"
)

// db2Cmd represents the db2 command
var db2Cmd = &cobra.Command{
	Use:   "db2 [flags] db2_path",
	Args:  cobra.ExactArgs(1),
	Short: "Parse db2 files",
	Long:  `Print information about db2 files and even list records`,
	Run: func(cmd *cobra.Command, args []string) {
		records, err := cmd.Flags().GetBool("records")
		if err != nil {
			panic("wtf")
		}
		dbd_path, err := cmd.Flags().GetString("dbd")
		if err != nil {
			panic("wtf")
		}

		f, err := os.Open(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open file: %v\n", err)
			os.Exit(2)
		}
		defer f.Close()

		db2, err := blizzard.OpenDB2File(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open db2: %v\n", err)
			os.Exit(3)
		}
		defer db2.Close()

		if dbd_path == "" {
			if records {
				printRecords(*db2)
			} else {
				printDB2Info(*db2)
			}
		} else {
			table, err := blizzard.DBDTableFromPath(dbd_path, db2)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to open dbd table: %v\n", err)
				os.Exit(3)
			}
			if records {
				printDBDRecords(*table)
			} else {
				printDBDInfo(*table)
			}
		}

	},
}

func init() {
	rootCmd.AddCommand(db2Cmd)
	db2Cmd.Flags().Bool("records", false, "Print all the records in the file")
	db2Cmd.Flags().String("dbd", "", "Include a dbd file")
}

func printDB2Info(db2 blizzard.DB2File) {
	fmt.Printf("layout: %08X, schema: %s, flags: 0x%X\n", db2.GetLayoutHash(), db2.GetSchema(), db2.Header.Flags)
	fmt.Printf("%d section(s), %d records of %d bytes with %d fields\n", len(db2.Sections), db2.Header.RecordCount, db2.Header.RecordSize, db2.Header.FieldCount)
	fmt.Printf("%d bytes of pallet data, %d bytes of common data\n", len(db2.PalletData), len(db2.CommonData))

	if db2.HasNonInlineIDs() {
		fmt.Printf("field info: non inline ids\n")
	} else {
		fmt.Printf("field info: id idx(%d)\n", db2.Header.IDIndex)
	}

	for _, field := range db2.FieldStorageInfos {
		fmt.Printf("\toffset: %d bits, size: %d bits, type: %d\n", field.FieldOffsetBits, field.FieldSizeBits, field.StorageType)
	}

	fmt.Printf("sections:\n")
	for _, sec := range db2.Sections {
		fmt.Printf("\thash: 0x%X, records: %d, noninline ids: %d, rel data: %d bytes, copy entries: %d\n", sec.TactKeyHash, sec.RecordCount, sec.IDListSize/4, sec.RelationshipDataSize, sec.CopyTableCount)
	}
}

func printDBDInfo(table blizzard.DBDTable) {
	inline := "id not inlined"
	if table.Schema.IDInlined {
		inline = "id inlined"
	}
	fmt.Printf("DBD Schema: %s\n", inline)
	for i, column := range table.Schema.Columns {
		fmt.Printf("\t%s: %s", column.Name, dbdFieldTypeString(column.FieldType))
		if i == table.Schema.IDIndex {
			fmt.Printf(" (id)")
		}
		if i == table.Schema.RelationIndex {
			fmt.Printf(" (rel)")
		}
		fmt.Printf("\n")
	}
}

func printRecords(db2 blizzard.DB2File) {
	for record := range db2.FixedRecords {
		fmt.Printf("%d:", record.GetID())
		for index := range record.NumFields() {
			field := record.GetField(index)
			switch field := field.(type) {
			case []byte:
				fmt.Printf(" %X", field)
			case int64:
				fmt.Printf(" %d", field)
			case uint64:
				fmt.Printf(" %d", field)
			case []uint32:
				fmt.Printf(" [%d", field[0])
				for _, v := range field[1:] {
					fmt.Printf(", %d", v)
				}
				fmt.Printf("]")
			}
		}
		fmt.Printf("\n")
	}
}

func printDBDRecords(table blizzard.DBDTable) {
	for record := range table.GetRecords {
		fmt.Printf("%d:", record.GetID())
		for index := range record.GetNumFields() {
			field := record.GetField(index)
			switch field := field.(type) {
			case uint8:
				fmt.Printf(" %d", field)
			case int8:
				fmt.Printf(" %d", field)
			case uint16:
				fmt.Printf(" %d", field)
			case int16:
				fmt.Printf(" %d", field)
			case uint32:
				fmt.Printf(" %d", field)
			case int32:
				fmt.Printf(" %d", field)
			case uint64:
				fmt.Printf(" %d", field)
			case int64:
				fmt.Printf(" %d", field)
			case string:
				fmt.Printf(" \"%s\"", field)
			case float32:
				fmt.Printf(" %f", field)
			case []uint16:
				fmt.Printf(" [%d", field[0])
				for _, v := range field[1:] {
					fmt.Printf(", %d", v)
				}
				fmt.Printf("]")
			case []int16:
				fmt.Printf(" [%d", field[0])
				for _, v := range field[1:] {
					fmt.Printf(", %d", v)
				}
				fmt.Printf("]")
			case []uint32:
				fmt.Printf(" [%d", field[0])
				for _, v := range field[1:] {
					fmt.Printf(", %d", v)
				}
				fmt.Printf("]")
			case []int32:
				fmt.Printf(" [%d", field[0])
				for _, v := range field[1:] {
					fmt.Printf(", %d", v)
				}
				fmt.Printf("]")
			case []float32:
				fmt.Printf(" [%f", field[0])
				for _, v := range field[1:] {
					fmt.Printf(", %f", v)
				}
				fmt.Printf("]")
			default:
				panic("unhandled field type")
			}
		}
		fmt.Printf("\n")
	}
}

func dbdFieldTypeString(fieldType blizzard.DBDFieldType) string {
	switch fieldType {
	case blizzard.U8:
		return "u8"
	case blizzard.S8:
		return "s8"
	case blizzard.U16:
		return "u16"
	case blizzard.S16:
		return "s16"
	case blizzard.U32:
		return "u32"
	case blizzard.S32:
		return "s32"
	case blizzard.U64:
		return "u64"
	case blizzard.S64:
		return "s64"
	case blizzard.Float:
		return "float"
	case blizzard.String:
		return "string"
	case blizzard.LocString:
		return "loc-string"
	}
	panic("unknown dbd field type")
}
