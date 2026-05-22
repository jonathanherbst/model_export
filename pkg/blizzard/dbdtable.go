package blizzard

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"reflect"
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

func (table DBDTable) Close() {
	table.database.Close()
}

func (table DBDTable) GetRecords(yield func(DBDRecord) bool) {
	for db2_record := range table.database.FixedRecords {
		record := DBDRecord{table.Schema, db2_record}
		if !yield(record) {
			return
		}
	}
}

func (table DBDTable) GetFixedRecordById(id uint32) *DBDRecord {
	db2_record := table.database.GetFixedRecordById(id)
	if db2_record != nil {
		return &DBDRecord{table.Schema, *db2_record}
	}
	return nil
}

func (table DBDTable) GetFixedRecordsByForeignKey(id uint32) func(func(DBDRecord) bool) {
	return func(yield func(DBDRecord) bool) {
		for db2_record := range table.database.GetFixedRecordsByForeignKey(id) {
			record := DBDRecord{table.Schema, db2_record}
			if !yield(record) {
				return
			}
		}
	}
}

func (table DBDTable) Cache() CachedDBDTable {
	cache := CachedDBDTable{
		records:    make(map[uint32]DBDRecord),
		foregnKeys: make(map[uint32][]uint32),
	}

	for section := range table.database.GetSections {
		for record := range section.FixedRecords {
			cache.records[record.GetID()] = DBDRecord{table.Schema, record}
		}
		for key, ids := range section.foreignKeyMap {
			if section.parent.HasNonInlineIDs() {
				cache.foregnKeys[key] = ids
			} else {
				cache.foregnKeys[key] = make([]uint32, len(ids))
				for i, idx := range ids {
					cache.foregnKeys[key][i] = section.GetFixedRecord(int(idx)).GetID()
				}
			}
		}
	}

	return cache
}

type CachedDBDTable struct {
	records    map[uint32]DBDRecord
	foregnKeys map[uint32][]uint32
}

func (table CachedDBDTable) GetFixedRecordById(id uint32) *DBDRecord {
	if record, ok := table.records[id]; ok {
		return &record
	}
	return nil
}

func (table CachedDBDTable) GetRecords() func(func(DBDRecord) bool) {
	return func(yield func(DBDRecord) bool) {
		for _, record := range table.records {
			if !yield(record) {
				return
			}
		}
	}
}

func (table CachedDBDTable) GetFixedRecordsByForeignKey(id uint32) func(func(DBDRecord) bool) {
	return func(yield func(DBDRecord) bool) {
		for _, recordId := range table.foregnKeys[id] {
			if record, ok := table.records[recordId]; ok {
				if !yield(record) {
					return
				}
			} else {
				panic("foreign key points to unknown record")
			}
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

func (r DBDRecord) GetFieldByName(name string) interface{} {
	idx, err := r.Schema.GetIndexByName(name)
	if err != nil {
		panic("no such field name")
	}
	if idx == r.Schema.IDIndex {
		return r.GetID()
	} else if idx < r.Schema.IDIndex {
		return r.GetField(idx)
	} else {
		return r.GetField(idx - 1)
	}
}

func (r DBDRecord) GetStringFieldByName(name string) string {
	switch f := r.GetFieldByName(name).(type) {
	case string:
		return f
	default:
		panic("field type is not a string")
	}
}

func (r DBDRecord) GetIntFieldByName(name string) int64 {
	switch f := r.GetFieldByName(name).(type) {
	case int64:
		return f
	case uint64:
		return int64(f)
	case uint32:
		return int64(f)
	default:
		panic("field type is not an integer")
	}
}

// Can return an int64, a uint64, a string, a []int64, or a []uint64
func (r DBDRecord) GetField(index int) interface{} {
	column := r.Schema.GetColumn(index)
	db2_field := r.record.GetField(index)
	switch f := db2_field.(type) {
	case int64:
		switch column.FieldType {
		case S8:
			return f
		case S16:
			return f
		case S32:
			return f
		case S64:
			return f
		case String:
			return r.record.GetFieldAsString(index)
		case LocString:
			return r.record.GetFieldAsString(index)
		default:
			panic("unknown type conversion")
		}
	case uint64:
		switch column.FieldType {
		case U8:
			return f
		case U16:
			return f
		case U32:
			return f
		case U64:
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
	case uint32:
		switch column.FieldType {
		case U8:
			return uint64(f)
		case U16:
			return uint64(f)
		case U32:
			return uint64(f)
		case S8:
			return int64(int8(f))
		case S16:
			return int64(int16(f))
		case S32:
			return int64(int32(f))
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
			return bytesToUnsignedInts[uint8](f, column.ArrayLen)
		case S8:
			return bytesToSignedInts[int8](f, column.ArrayLen)
		case U16:
			return bytesToUnsignedInts[uint16](f, column.ArrayLen)
		case S16:
			return bytesToSignedInts[int16](f, column.ArrayLen)
		case U32:
			return bytesToUnsignedInts[uint32](f, column.ArrayLen)
		case S32:
			return bytesToSignedInts[int32](f, column.ArrayLen)
		case U64:
			return bytesToUnsignedInts[uint64](f, column.ArrayLen)
		case S64:
			return bytesToSignedInts[int64](f, column.ArrayLen)
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
			seq := make([]uint64, len(f))
			for i, c := range f {
				seq[i] = uint64(c)
			}
			return seq
		case S16:
			seq := make([]int64, len(f))
			for i, c := range f {
				seq[i] = int64(int16(c))
			}
			return seq
		case U32:
			seq := make([]uint64, len(f))
			for i, c := range f {
				seq[i] = uint64(c)
			}
			return seq
		case S32:
			seq := make([]int64, len(f))
			for i, c := range f {
				seq[i] = int64(int32(c))
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

func GetSliceFieldByName[T any](record DBDRecord, name string) []T {
	field := record.GetFieldByName(name)
	switch f := field.(type) {
	case []T:
		return f
	default:
		panic("unexpected field type")
	}
}

// bytesToUnsignedInts converts a byte slice into a slice of uint64, parsing as little endian.
// T must be an unsigned integer type (8 to 64 bits) that determines the parsing size.
// Panics if the byte array size does not match the expected count.
func bytesToUnsignedInts[T ~uint8 | ~uint16 | ~uint32 | ~uint64](data []byte, arrayLen int) interface{} {
	if arrayLen == 0 {
		return bytesToUnsignedInt[T](data)
	}

	var zero T
	typ := reflect.TypeOf(zero)
	size := int(typ.Size())
	if len(data) != arrayLen*size {
		panic("byte array size does not match expected count")
	}
	result := make([]uint64, 0, arrayLen)
	for chunk := range slices.Chunk(data, size) {
		result = append(result, bytesToUnsignedInt[T](chunk))
	}
	return result
}

func bytesToUnsignedInt[T ~uint8 | ~uint16 | ~uint32 | ~uint64](data []byte) uint64 {
	var zero T
	typ := reflect.TypeOf(zero)
	switch typ.Kind() {
	case reflect.Uint8:
		return uint64(data[0])
	case reflect.Uint16:
		return uint64(binary.LittleEndian.Uint16(data))
	case reflect.Uint32:
		return uint64(binary.LittleEndian.Uint32(data))
	case reflect.Uint64:
		return binary.LittleEndian.Uint64(data)
	default:
		panic("unsupported type")
	}
}

// bytesToSignedInts converts a byte slice into a slice of int64, parsing as little endian.
// T must be a signed integer type (8 to 64 bits) that determines the parsing size.
// Panics if the byte array size does not match the expected count.
func bytesToSignedInts[T ~int8 | ~int16 | ~int32 | ~int64](data []byte, arrayLen int) interface{} {
	if arrayLen == 0 {
		return bytesToSignedInt[T](data)
	}

	var zero T
	typ := reflect.TypeOf(zero)
	size := int(typ.Size())
	if len(data) != arrayLen*size {
		panic("byte array size does not match expected count")
	}
	result := make([]int64, 0, arrayLen)
	for chunk := range slices.Chunk(data, size) {
		result = append(result, bytesToSignedInt[T](chunk))
	}
	return result
}

func bytesToSignedInt[T ~int8 | ~int16 | ~int32 | ~int64](data []byte) int64 {
	var zero T
	typ := reflect.TypeOf(zero)
	switch typ.Kind() {
	case reflect.Int8:
		return int64(int8(data[0]))
	case reflect.Int16:
		return int64(int16(binary.LittleEndian.Uint16(data)))
	case reflect.Int32:
		return int64(int32(binary.LittleEndian.Uint32(data)))
	case reflect.Int64:
		return int64(binary.LittleEndian.Uint64(data))
	default:
		panic("unsupported type")
	}
}
