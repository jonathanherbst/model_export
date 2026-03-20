package blizzard

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
)

// WDC5 DB2 Header
type WDC5Header struct {
	Magic                [4]byte   // 'WDC5'
	Version              uint32    // 5, probably numeric version?
	SchemaString         [128]byte // "WowStatic_Patch_10_2_5" + padding in 10.2.5.52432
	RecordCount          uint32    // this is for all sections combined now
	FieldCount           uint32
	RecordSize           uint32
	StringTableSize      uint32 // this is for all sections combined now
	TableHash            uint32 // hash of the table name
	LayoutHash           uint32 // this is a hash field that changes only when the structure of the data changes
	MinID                uint32
	MaxID                uint32
	Locale               uint32 // as seen in TextWowEnum
	Flags                uint16 // possible values are listed in Known Flag Meanings
	IDIndex              uint16 // this is the index of the field containing ID values; this is ignored if flags & 0x04 != 0
	TotalFieldCount      uint32 // from WDC1 onwards, this value seems to always be the same as the 'field_count' value
	BitpackedDataOffset  uint32 // relative position in record where bitpacked data begins; not important for parsing the file
	LookupColumnCount    uint32
	FieldStorageInfoSize uint32
	CommonDataSize       uint32
	PalletDataSize       uint32
	SectionCount         uint32 // new to WDC2, this is number of sections of data
}

// WDC5 Section Header
type WDC5SectionHeader struct {
	TactKeyHash          uint64 // TactKeyLookup hash
	FileOffset           uint32 // absolute position to the beginning of the section
	RecordCount          uint32 // 'record_count' for the section
	StringTableSize      uint32 // 'string_table_size' for the section
	OffsetRecordsEnd     uint32 // Offset to the spot where the records end in a file with an offset map structure;
	IDListSize           uint32 // Size of the list of ids present in the section
	RelationshipDataSize uint32 // Size of the relationship data in the section
	OffsetMapIDCount     uint32 // Count of ids present in the offset map in the section
	CopyTableCount       uint32 // Count of the number of deduplication entries
}

// Field structure for WDC5
type WDC5FieldStructure struct {
	Size     int16  // size in bits as calculated by: byteSize = (32 - size) / 8; this value can be negative to indicate field sizes larger than 32-bits
	Position uint16 // position of the field within the record, relative to the start of the record
}

// Field compression types for WDC5
type WDC5FieldCompression int32

const (
	FieldCompressionNone                  WDC5FieldCompression = 0 // None -- usually the field is a 8-, 16-, 32-, or 64-bit integer in the record data. But can contain 96-bit value representing 3 floats as well
	FieldCompressionBitpacked             WDC5FieldCompression = 1 // Bitpacked -- the field is a bitpacked integer in the record data.
	FieldCompressionCommonData            WDC5FieldCompression = 2 // Common data -- the field is assumed to be a default value, and exceptions from that default value are stored in the corresponding section in common_data
	FieldCompressionBitpackedIndexed      WDC5FieldCompression = 3 // Bitpacked indexed -- the field has a bitpacked index in the record data.
	FieldCompressionBitpackedIndexedArray WDC5FieldCompression = 4 // Bitpacked indexed array -- the field has a bitpacked index in the record data.
	FieldCompressionBitpackedSigned       WDC5FieldCompression = 5 // Same as field_compression_bitpacked
)

// Field storage info for WDC5
type WDC5FieldStorageInfo struct {
	FieldOffsetBits    uint16
	FieldSizeBits      uint16 // very important for reading bitpacked fields; size is the sum of all array pieces in bits - for example, uint32[3] will appear here as '96'
	AdditionalDataSize uint32 // the size in bytes of the corresponding section in common_data or pallet_data. These sections are in the same order as the field_info, so to find the offset, add up the additional_data_size of any previous fields which are stored in the same block
	StorageType        WDC5FieldCompression
	Data               [3]uint32 // Data fields depend on storage_type
}

type WDC5FieldDataBitpacked struct {
	OffsetBits uint32 // not useful for most purposes; formula they use to calculate is bitpacking_offset_bits = field_offset_bits - (header.bitpacked_data_offset * 8)
	SizeBits   uint32 // not useful for most purposes
	Flags      uint32 // known values - 0x01: sign-extend (signed)
}

type WDC5FieldDataBitpackedIndex struct {
	OffsetBits uint32 // not useful for most purposes; formula they use to calculate is bitpacking_offset_bits = field_offset_bits - (header.bitpacked_data_offset * 8)
	SizeBits   uint32 // not useful for most purposes
	ArrayCount uint32
}

type WDC5FieldDataCommonData struct {
	Default uint32
}

func (info WDC5FieldStorageInfo) BitpackedParams() WDC5FieldDataBitpacked {
	if info.StorageType != FieldCompressionBitpacked && info.StorageType != FieldCompressionBitpackedSigned {
		panic("trying to get bitpacked data when not a bitpacked type")
	}
	return WDC5FieldDataBitpacked{
		OffsetBits: info.Data[0],
		SizeBits:   info.Data[1],
		Flags:      info.Data[2],
	}
}

func (info WDC5FieldStorageInfo) BitpackedIndexParams() WDC5FieldDataBitpackedIndex {
	if info.StorageType != FieldCompressionBitpackedIndexed && info.StorageType != FieldCompressionBitpackedIndexedArray {
		panic("trying to get bitpacked index data when not a bitpacked index type")
	}
	return WDC5FieldDataBitpackedIndex{
		OffsetBits: info.Data[0],
		SizeBits:   info.Data[1],
		ArrayCount: info.Data[2],
	}
}

func (info WDC5FieldStorageInfo) CommonDataParams() WDC5FieldDataCommonData {
	if info.StorageType != FieldCompressionCommonData {
		panic("trying to get common data when not a common data type")
	}
	return WDC5FieldDataCommonData{Default: info.Data[0]}
}

// Holds encryption status of specific ID in a section where tact_key_hash is not 0
type WDC5EncryptedStatus struct {
	EncryptedIDCount int32
	// Followed by: encrypted_id[encrypted_id_count]
}

// Copy table entry for deduplication
type WDC5CopyTableEntry struct {
	IDOfNewRow    uint32
	IDOfCopiedRow uint32
}

// Offset map entry
type WDC5OffsetMapEntry struct {
	Offset uint32
	Size   uint16
}

// Relationship entry
type WDC5RelationshipEntry struct {
	ForeignID   uint32 // This is the id of the foreign key for the record, e.g. SpellID in SpellX* tables.
	RecordIndex uint32 // This is the index of the record in record_data. Note that this is *not* the record's own ID *unless* flag 0x02 is set.
}

// Relationship mapping
type WDC5RelationshipMapping struct {
	NumEntries uint32
	MinID      uint32
	MaxID      uint32
	// Followed by: entries[num_entries]
}

// Common data map entry
type WDC5CommonDataMapEntry struct {
	ID       uint32
	RawValue uint32
}

// Known flag meanings for WDC5
const (
	FlagHasOffsetMap        uint16 = 0x01 // 'Has offset map'
	FlagHasRelationshipData uint16 = 0x02 // 'Has relationship data'
	FlagHasNonInlineIDs     uint16 = 0x04 // 'Has non-inline IDs'
	FlagIsBitpacked         uint16 = 0x10 // 'Is bitpacked'
)

// Open opens a WDC5 DB2 file from a reader
func OpenDB2File(reader io.ReadSeekCloser) (*DB2File, error) {
	// start at the beginning
	_, err := reader.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("unable to seek db2 file: %w", err)
	}

	file := &DB2File{reader: reader}

	// Read header
	if err := binary.Read(reader, binary.LittleEndian, &file.Header); err != nil {
		return nil, errors.New("invalid DB2 file")
	}

	// Verify magic
	if string(file.Header.Magic[:]) != "WDC5" {
		return nil, errors.New("not a WDC5 db2")
	}

	file.Sections = make([]WDC5SectionHeader, file.Header.SectionCount)
	for i := range file.Sections {
		if err := binary.Read(reader, binary.LittleEndian, &file.Sections[i]); err != nil {
			return nil, errors.New("invalid WDC5 header")
		}
	}

	file.FieldStructures = make([]WDC5FieldStructure, file.Header.TotalFieldCount)
	for i := range file.FieldStructures {
		if err := binary.Read(reader, binary.LittleEndian, &file.FieldStructures[i]); err != nil {
			return nil, errors.New("invalid WDC5 header")
		}
	}

	// Read field storage infos
	file.FieldStorageInfos = make([]WDC5FieldStorageInfo, file.Header.FieldCount)
	for i := range file.FieldStorageInfos {
		if err := binary.Read(reader, binary.LittleEndian, &file.FieldStorageInfos[i]); err != nil {
			return nil, errors.New("invalid WDC5 header")
		}
	}

	// Read pallet data
	file.PalletData = make([]uint32, file.Header.PalletDataSize/4)
	if err := binary.Read(reader, binary.LittleEndian, &file.PalletData); err != nil {
		return nil, errors.New("invalid WDC5 header")
	}

	// Read common data
	file.CommonData = make(map[uint32]uint32)
	for _ = range file.Header.CommonDataSize / 8 {
		var entry WDC5CommonDataMapEntry
		if err := binary.Read(reader, binary.LittleEndian, &entry); err != nil {
			return nil, errors.New("invalid WDC5 header")
		}
		file.CommonData[entry.ID] = entry.RawValue
	}
	return file, nil
}

// File represents a WDC5 DB2 file
type DB2File struct {
	reader            io.ReadSeekCloser
	Header            WDC5Header
	Sections          []WDC5SectionHeader
	FieldStructures   []WDC5FieldStructure
	FieldStorageInfos []WDC5FieldStorageInfo
	PalletData        []uint32
	CommonData        map[uint32]uint32
}

func (file DB2File) Close() {
	file.reader.Close()
}

func (file DB2File) GetSchema() string {
	return stringFromNullTermBytes(file.Header.SchemaString[:])
}

func (file DB2File) GetLayoutHash() uint32 {
	return file.Header.LayoutHash
}

func (file DB2File) HasVariableRecords() bool {
	return ((file.Header.Flags) & FlagHasOffsetMap) != 0
}

func (file DB2File) HasRelationshipData() bool {
	return (file.Header.Flags & FlagHasRelationshipData) != 0
}

func (file DB2File) HasNonInlineIDs() bool {
	return (file.Header.Flags & FlagHasNonInlineIDs) != 0
}

func (file *DB2File) FixedRecords(yield func(DB2FixedRecord) bool) {
	for section := range file.GetSections {
		section.FixedRecords(yield)
	}
}

func (file *DB2File) GetFixedRecordById(id uint32) *DB2FixedRecord {
	for section := range file.GetSections {
		record := section.GetFixedRecordById(id)
		if record != nil {
			return record
		}
	}
	return nil
}

func (file *DB2File) GetFixedRecordsByForeignKey(id uint32, yield func(DB2FixedRecord) bool) {
	for section := range file.GetSections {
		for record := range func(yield func(DB2FixedRecord) bool) { section.GetFixedRecordsByForeignKey(id, yield) } {
			if !yield(record) {
				return
			}
		}
	}
}

// Iterate all sections in the file
func (file *DB2File) GetSections(yield func(DB2Section) bool) {
	for _, header := range file.Sections {
		if header.TactKeyHash != 0 {
			// we don't know how to handle encrypted sections yet
			continue
		}

		if _, err := file.reader.Seek(int64(header.FileOffset), 0); err != nil {
			continue
		}

		section := DB2Section{parent: file}

		section.numRecords = header.RecordCount
		recordRegionSize := (header.RecordCount * file.Header.RecordSize) + header.StringTableSize
		if file.HasVariableRecords() {
			recordRegionSize = header.OffsetRecordsEnd - header.FileOffset
		}
		section.recordRegion = make([]byte, recordRegionSize)
		if s, err := file.reader.Read(section.recordRegion); err != nil || s != int(recordRegionSize) {
			continue
		}

		section.idList = make([]uint32, header.IDListSize/4)
		if err := binary.Read(file.reader, binary.LittleEndian, section.idList); err != nil {
			continue
		}

		section.copyTable = make(map[uint32]uint32)
		for _ = range header.CopyTableCount {
			var entry WDC5CopyTableEntry
			if err := binary.Read(file.reader, binary.LittleEndian, &entry); err != nil {
				break
			}
			section.copyTable[entry.IDOfNewRow] = entry.IDOfCopiedRow
		}

		section.offsetEntries = make([]WDC5OffsetMapEntry, header.OffsetMapIDCount)
		for i := range section.offsetEntries {
			if err := binary.Read(file.reader, binary.LittleEndian, &section.offsetEntries[i]); err != nil {
				break
			}
		}

		if file.HasRelationshipData() {
			section.offsetIds = make([]uint32, header.OffsetMapIDCount)
			if err := binary.Read(file.reader, binary.LittleEndian, section.offsetIds); err != nil {
				continue
			}
		}

		section.foreignKeyMap = make(map[uint32][]uint32)
		if header.RelationshipDataSize > 0 {
			var mapping WDC5RelationshipMapping
			if err := binary.Read(file.reader, binary.LittleEndian, &mapping); err != nil {
				continue
			}
			section.foreignKeyMinId = mapping.MinID
			section.foreignKeyMaxId = mapping.MaxID
			for _ = range mapping.NumEntries {
				var entry WDC5RelationshipEntry
				if err := binary.Read(file.reader, binary.LittleEndian, &entry); err != nil {
					break
				}
				if entries, ok := section.foreignKeyMap[entry.ForeignID]; ok {
					section.foreignKeyMap[entry.ForeignID] = append(entries, entry.RecordIndex)
				} else {
					section.foreignKeyMap[entry.ForeignID] = make([]uint32, 0, 1)
					section.foreignKeyMap[entry.ForeignID] = append(section.foreignKeyMap[entry.ForeignID], entry.RecordIndex)
				}

			}
		}

		// this comes before the relationship mapping if HasRelationshipData
		if !file.HasRelationshipData() {
			section.offsetIds = make([]uint32, header.OffsetMapIDCount)
			if err := binary.Read(file.reader, binary.LittleEndian, section.offsetIds); err != nil {
				continue
			}
		}

		if !yield(section) {
			break
		}
	}
}

// Section represents a section in the DB2 file
type DB2Section struct {
	parent          *DB2File
	recordRegion    []byte
	idList          []uint32
	copyTable       map[uint32]uint32
	offsetEntries   []WDC5OffsetMapEntry
	offsetIds       []uint32
	foreignKeyMinId uint32
	foreignKeyMaxId uint32
	foreignKeyMap   map[uint32][]uint32
	numRecords      uint32
}

func (section *DB2Section) FixedRecords(yield func(DB2FixedRecord) bool) {
	for idx := 0; idx < int(section.numRecords); idx++ {
		if !yield(section.GetFixedRecord(idx)) {
			break
		}
	}
}

func (section *DB2Section) GetFixedRecordById(id uint32) *DB2FixedRecord {
	if section.parent.HasNonInlineIDs() {
		if copied_id, ok := section.copyTable[id]; ok {
			id = copied_id
		}
		idx := sort.Search(len(section.idList), func(i int) bool { return section.idList[i] >= id })
		if idx < len(section.idList) && section.idList[idx] == id {
			record := section.GetFixedRecord(idx)
			return &record
		}
	} else {
		for r := range section.FixedRecords {
			if r.GetID() == id {
				return &r
			}
		}
	}
	return nil
}

func (section *DB2Section) GetFixedRecordsByForeignKey(fk uint32, yield func(DB2FixedRecord) bool) {
	if entries, ok := section.foreignKeyMap[fk]; ok {
		if section.parent.HasRelationshipData() {
			for _, id := range entries {
				record := section.GetFixedRecordById(id)
				if record != nil && !yield(*record) {
					return
				}
			}
		} else {
			for _, idx := range entries {
				if !yield(section.GetFixedRecord(int(idx))) {
					return
				}
			}
		}
	}
}

func (section *DB2Section) GetFixedRecord(idx int) DB2FixedRecord {
	recordSize := int(section.parent.Header.RecordSize)
	record := DB2FixedRecord{
		section,
		newWDC5BitBuffer(section.recordRegion[idx*recordSize : (idx+1)*recordSize]),
		nil,
		uint(idx),
	}
	if section.parent.HasNonInlineIDs() {
		record.id = &section.idList[idx]
	}
	return record
}

func (section DB2Section) getString(index uint) string {
	return stringFromNullTermBytes(section.recordRegion[index:])
}

// FixedRecord represents a fixed record
type DB2FixedRecord struct {
	parent *DB2Section
	data   wdc5BitBuffer
	id     *uint32
	index  uint
}

// GetID returns the ID of the record
func (r DB2FixedRecord) GetID() uint32 {
	if r.id != nil {
		return *r.id
	}
	idField := r.getFieldWithID(int(r.parent.parent.Header.IDIndex))
	switch f := idField.(type) {
	case int64:
		return uint32(f)
	case uint64:
		return uint32(f)
	default:
		panic("ID is not an integer type")
	}
}

// NumFields returns the number of fields
func (r DB2FixedRecord) NumFields() int {
	if r.id == nil {
		return len(r.fields()) - 1
	}
	return len(r.fields())
}

// GetField returns the field at the given index
func (r DB2FixedRecord) GetField(index int) interface{} {
	return r.getFieldWithID(r.getFieldIndexWithID(index))
}

// GetFieldAsString returns the field as a string
func (r DB2FixedRecord) GetFieldAsString(index int) string {
	field := r.GetField(index)
	var stringIndex uint32
	switch f := field.(type) {
	case []byte:
		if len(f) == 4 {
			stringIndex = binary.LittleEndian.Uint32(f)
		} else {
			panic(fmt.Sprintf("unknown conversion from bytes of len %d to string offset", len(f)))
		}
	case int64:
		stringIndex = uint32(f)
	case uint64:
		stringIndex = uint32(f)
	default:
		panic("unexpected type when trying to get a string")
	}

	if stringIndex == 0 {
		return ""
	}
	// String indexes are referenced from where the field begins in the record.
	// Calculate the index from the beginning of the record.
	fieldIndex := r.getFieldIndexWithID(index)
	recordIndex := uint(stringIndex) + r.index*uint(r.parent.parent.Header.RecordSize) + uint(r.fields()[fieldIndex].FieldOffsetBits/8)
	// get_string indexes from the string block
	return r.parent.getString(recordIndex)
}

func (r DB2FixedRecord) getFieldIndexWithID(index int) int {
	if r.id == nil && index >= int(r.parent.parent.Header.IDIndex) {
		return index + 1
	}
	return index
}

// getFieldWithID gets the field with the given ID
func (r DB2FixedRecord) getFieldWithID(index int) interface{} {
	field := r.fields()[index]
	switch field.StorageType {
	case FieldCompressionNone:
		offset := field.FieldOffsetBits / 8
		size := field.FieldSizeBits / 8
		return r.data.inner[offset : offset+size]
	case FieldCompressionBitpacked:
		return r.data.GetUnsigned(uint(field.FieldOffsetBits), uint(field.FieldSizeBits))
	case FieldCompressionBitpackedSigned:
		return r.data.GetSigned(uint(field.FieldOffsetBits), uint(field.FieldSizeBits))
	case FieldCompressionBitpackedIndexed:
		palletIndex := r.data.GetUnsigned(uint(field.FieldOffsetBits), uint(field.FieldSizeBits))
		return r.parent.parent.PalletData[palletIndex]
	case FieldCompressionBitpackedIndexedArray:
		params := field.BitpackedIndexParams()
		palletIndex := r.data.GetUnsigned(uint(field.FieldOffsetBits), uint(field.FieldSizeBits))
		return r.parent.parent.PalletData[palletIndex : palletIndex+uint64(params.ArrayCount)]
	case FieldCompressionCommonData:
		if val, ok := r.parent.parent.CommonData[r.GetID()]; ok {
			return val
		}
		return field.CommonDataParams().Default
	}
	panic("unknown field storage type")
}

func (r DB2FixedRecord) fields() []WDC5FieldStorageInfo {
	return r.parent.parent.FieldStorageInfos
}

func newWDC5BitBuffer(buffer []byte) wdc5BitBuffer {
	return wdc5BitBuffer{inner: buffer}
}

// wdc5BitBuffer for bit-level operations
type wdc5BitBuffer struct {
	inner []byte
}

// GetUnsigned gets an unsigned value from the buffer
func (b wdc5BitBuffer) GetUnsigned(index, size uint) uint64 {
	if size <= 0 || size >= 64 {
		panic("invalid size")
	}
	var data [8]byte
	sizeBytes := (size + (index & 7) + 7) / 8
	indexBytes := index / 8
	copy(data[:], b.inner[indexBytes:indexBytes+sizeBytes])
	dataU64 := binary.LittleEndian.Uint64(data[:])
	return (dataU64 >> uint(index&7)) & ((uint64(1) << uint(size)) - 1)
}

// GetSigned gets a signed value from the buffer
func (b wdc5BitBuffer) GetSigned(index, size uint) int64 {
	value := b.GetUnsigned(index, size)
	// Do bit extend if needed
	signMask := uint64(1) << uint(size-1)
	if (value & signMask) != 0 {
		value |= ^((uint64(1) << uint(size)) - 1)
	}
	return int64(value)
}

func stringFromNullTermBytes(buf []byte) string {
	nullIndex := bytes.IndexByte(buf, 0)
	if nullIndex == -1 {
		return ""
	}
	return string(buf[:nullIndex])
}
