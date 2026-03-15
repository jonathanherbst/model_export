package blizzard

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"slices"
)

func DBDTableFromPath(path string, database *DB2File) (*DBDTable, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return DBDTableFromReader(f, database)
}

func DBDTableFromReader(reader io.Reader, database *DB2File) (*DBDTable, error) {
	sel := DBDLayoutHashSelector(database.GetLayoutHash())
	schema, err := DBDFromReader(reader, sel)
	if err != nil {
		return nil, err
	}
	if schema.IDIndex != int(database.Header.IDIndex) {
		return nil, errors.New("schema and db2 don't match")
	}
	return &DBDTable{*schema, database}, nil
}

type DBDTable struct {
	Schema   DBDSchema
	database *DB2File
}

func (table DBDTable) GetRecords(yield func(DBDRecord) bool) {
	for db2_record := range table.database.FixedRecords {
		record := DBDRecord{table.Schema, db2_record}
		if !yield(record) {
			return
		}
	}
}

type DBDRecord struct {
	Schema DBDSchema
	record DB2FixedRecord
}

func (r DBDRecord) GetID() uint32 {
	return r.record.GetID()
}

func (r DBDRecord) GetNumFields() int {
	return r.record.NumFields()
}

func (r DBDRecord) GetField(index int) interface{} {
	column := r.Schema.GetColumn(index)
	db2_field := r.record.GetField(index)
	switch f := db2_field.(type) {
	case int64:
		switch column.FieldType {
		case S8:
			return int8(f)
		case S16:
			return int16(f)
		case S32:
			return int32(f)
		case S64:
			return f
		case String:
		case LocString:
			return r.record.GetFieldAsString(index)
		default:
			panic("unknown type conversion")
		}
	case uint64:
		switch column.FieldType {
		case U8:
			return uint8(f)
		case U16:
			return uint16(f)
		case U32:
			return uint32(f)
		case U64:
			return f
		case Float:
			return math.Float32frombits(uint32(f))
		case String:
		case LocString:
			return r.record.GetFieldAsString(index)
		default:
			panic("unknown type conversion")
		}
	case uint32:
		switch column.FieldType {
		case U8:
			return uint8(f)
		case U16:
			return uint16(f)
		case U32:
			return f
		case Float:
			return math.Float32frombits(uint32(f))
		case String:
			return r.record.GetFieldAsString(index)
		case LocString:
			return r.record.GetFieldAsString(index)
		default:
			panic("unknown type conversion")
		}
	case []byte:
		switch column.FieldType {
		case U8:
			if column.ArrayLen > 0 {
				return f
			} else {
				return f[0]
			}
		case S8:
			if column.ArrayLen > 0 {
				seq := make([]int8, len(f))
				for i, v := range f {
					seq[i] = int8(v)
				}
				return seq
			} else {
				return int8(f[0])
			}
		case U16:
			var seq []uint16
			for c := range slices.Chunk(f, 2) {
				seq = append(seq, binary.LittleEndian.Uint16(c))
			}
			if column.ArrayLen > 0 {
				return seq
			} else {
				return seq[0]
			}
		case S16:
			var seq []int16
			for c := range slices.Chunk(f, 2) {
				seq = append(seq, int16(binary.LittleEndian.Uint16(c)))
			}
			if column.ArrayLen > 0 {
				return seq
			} else {
				return seq[0]
			}
		case U32:
			var seq []uint32
			for c := range slices.Chunk(f, 4) {
				seq = append(seq, binary.LittleEndian.Uint32(c))
			}
			if column.ArrayLen > 0 {
				return seq
			} else {
				return seq[0]
			}
		case S32:
			var seq []int32
			for c := range slices.Chunk(f, 4) {
				seq = append(seq, int32(binary.LittleEndian.Uint32(c)))
			}
			if column.ArrayLen > 0 {
				return seq
			} else {
				return seq[0]
			}
		case U64:
			var seq []uint64
			for c := range slices.Chunk(f, 8) {
				seq = append(seq, binary.LittleEndian.Uint64(c))
			}
			if column.ArrayLen > 0 {
				return seq
			} else {
				return seq[0]
			}
		case S64:
			var seq []int64
			for c := range slices.Chunk(f, 8) {
				seq = append(seq, int64(binary.LittleEndian.Uint64(c)))
			}
			if column.ArrayLen > 0 {
				return seq
			} else {
				return seq[0]
			}
		case Float:
			var seq []float32
			for c := range slices.Chunk(f, 4) {
				seq = append(seq, math.Float32frombits(binary.LittleEndian.Uint32(c)))
			}
			if column.ArrayLen > 0 {
				return seq
			} else {
				return seq[0]
			}
		case String:
			return r.record.GetFieldAsString(index)
		case LocString:
			return r.record.GetFieldAsString(index)
		default:
			panic("unknown type conversion")
		}
	case []uint32:
		switch column.FieldType {
		case U16:
			seq := make([]uint16, len(f))
			for i, c := range f {
				seq[i] = uint16(c)
			}
			return seq
		case S16:
			seq := make([]int16, len(f))
			for i, c := range f {
				seq[i] = int16(c)
			}
			return seq
		case U32:
			return f
		case S32:
			seq := make([]uint32, len(f))
			for i, c := range f {
				seq[i] = uint32(c)
			}
			return seq
		case Float:
			seq := make([]float32, len(f))
			for i, c := range f {
				seq[i] = math.Float32frombits(c)
			}
			return seq
		default:
			panic("unknown type conversion")
		}
	}
	panic("unhandled field type")
}
