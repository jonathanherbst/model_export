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

// db2utilCmd represents the db2util command
var db2utilCmd = &cobra.Command{
	Use:   "db2util [flags] db2_path",
	Args:  cobra.ExactArgs(1),
	Short: "Parse db2 files",
	Long:  `Print information about db2 files and even list records`,
	Run: func(cmd *cobra.Command, args []string) {
		records, err := cmd.Flags().GetBool("records")
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

		if records {
			print_records(*db2)
		} else {
			print_db2_info(*db2)
		}
	},
}

func init() {
	rootCmd.AddCommand(db2utilCmd)
	db2utilCmd.Flags().Bool("records", false, "Print all the records in the file")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// db2utilCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// db2utilCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func print_db2_info(db2 blizzard.DB2File) {
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
	// var idx: usize = 0;
	// for (wdc5_file.field_storage_infos()) |field| {
	//     if (maybe_dbd_def) |dbd_def| {
	//         // skip all the noninline columns
	//         while (dbd_def.columns.items[idx].annotations.noninline) {
	//             idx += 1;
	//         }
	//         writer.print("\t{s}({}) - ", .{ dbd_def.columns.items[idx].name, dbd_def.columns.items[idx].field_type });
	//         idx += 1;
	//     } else {
	//         writer.print("\t", .{});
	//     }
	//     writer.print("offset: {} bits, size: {} bits, type: {}\n", .{ field.field_offset_bits, field.field_size_bits, field.storage_type });
	// }

	fmt.Printf("sections:\n")
	for _, sec := range db2.Sections {
		fmt.Printf("\thash: 0x%X, records: %d, noninline ids: %d, rel data: %d bytes, copy entries: %d\n", sec.TactKeyHash, sec.RecordCount, sec.IDListSize/4, sec.RelationshipDataSize, sec.CopyTableCount)
	}
}

func print_records(db2 blizzard.DB2File) {
	for record := range db2.FixedRecords {
		fmt.Printf("%d:", record.GetID())
		for index := range record.NumFields() {
			field := record.GetField(uint(index))
			switch field := field.(type) {
			case []byte:
				fmt.Printf(" %X", field)
			case int64:
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
		//for (0..record.num_fields()) |idx| {
		//             const field = record.get_field(idx);
		//             switch (field) {
		//                 .bytes => |v| stdout.print(" {X}", .{v}),
		//                 .indexed => |v| {
		//                     stdout.print(" [{}", .{v[0]});
		//                     for (v[1..]) |num| {
		//                         stdout.print(", {}", .{num});
		//                     }
		//                     stdout.print("]", .{});
		//                 },
		//                 .signed => |v| stdout.print(" {}", .{v}),
		//                 .unsigned => |v| stdout.print(" {}", .{v}),
		//             }
		//         }
	}

	// var records = try wdc5_file.records();
	// while (records.next()) |record| {
	//     stdout.print("{}:", .{record.get_id()});
	//     if (maybe_dbd_def) |dbd_def| {
	//         const dbd_record: dbd_db2.DefinedRecord = .{
	//             .record = record,
	//             .schema = dbd_def,
	//         };

	//         for (0..dbd_record.num_fields()) |idx| {
	//             const field = dbd_record.get_field(idx);
	//             if (field.num_values() == 1) {
	//                 switch (field.get_value(0)) {
	//                     .float => |v| stdout.print(" {}", .{v}),
	//                     .signed => |v| stdout.print(" {}", .{v}),
	//                     .unsigned => |v| stdout.print(" {}", .{v}),
	//                     .string => |v| stdout.print(" \"{s}\"", .{v}),
	//                 }
	//             } else {
	//                 switch (field.get_value(0)) {
	//                     .float => |v| stdout.print(" [{}", .{v}),
	//                     .signed => |v| stdout.print(" [{}", .{v}),
	//                     .unsigned => |v| stdout.print(" [{}", .{v}),
	//                     .string => |v| stdout.print(" [\"{s}\"", .{v}),
	//                 }
	//                 for (1..field.num_values()) |field_idx| {
	//                     switch (field.get_value(field_idx)) {
	//                         .float => |v| stdout.print(", {}", .{v}),
	//                         .signed => |v| stdout.print(", {}", .{v}),
	//                         .unsigned => |v| stdout.print(", {}", .{v}),
	//                         .string => |v| stdout.print(", \"{s}\"", .{v}),
	//                     }
	//                 }
	//                 stdout.print("]", .{});
	//             }
	//         }
	//     } else {
	//         for (0..record.num_fields()) |idx| {
	//             const field = record.get_field(idx);
	//             switch (field) {
	//                 .bytes => |v| stdout.print(" {X}", .{v}),
	//                 .indexed => |v| {
	//                     stdout.print(" [{}", .{v[0]});
	//                     for (v[1..]) |num| {
	//                         stdout.print(", {}", .{num});
	//                     }
	//                     stdout.print("]", .{});
	//                 },
	//                 .signed => |v| stdout.print(" {}", .{v}),
	//                 .unsigned => |v| stdout.print(" {}", .{v}),
	//             }
	//         }
	//     }
	//     stdout.print("\n", .{});
	// }
}
