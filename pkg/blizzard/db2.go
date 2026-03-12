package blizzard

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unsafe"
)

// WDC5 DB2 Header
type Header struct {
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
type SectionHeader struct {
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
type FieldStructure struct {
	Size     int16  // size in bits as calculated by: byteSize = (32 - size) / 8; this value can be negative to indicate field sizes larger than 32-bits
	Position uint16 // position of the field within the record, relative to the start of the record
}

// Field compression types for WDC5
type FieldCompression int32

const (
	FieldCompressionNone                  FieldCompression = 0 // None -- usually the field is a 8-, 16-, 32-, or 64-bit integer in the record data. But can contain 96-bit value representing 3 floats as well
	FieldCompressionBitpacked             FieldCompression = 1 // Bitpacked -- the field is a bitpacked integer in the record data.
	FieldCompressionCommonData            FieldCompression = 2 // Common data -- the field is assumed to be a default value, and exceptions from that default value are stored in the corresponding section in common_data
	FieldCompressionBitpackedIndexed      FieldCompression = 3 // Bitpacked indexed -- the field has a bitpacked index in the record data.
	FieldCompressionBitpackedIndexedArray FieldCompression = 4 // Bitpacked indexed array -- the field has a bitpacked index in the record data.
	FieldCompressionBitpackedSigned       FieldCompression = 5 // Same as field_compression_bitpacked
)

// Field storage info for WDC5
type FieldStorageInfo struct {
	FieldOffsetBits    uint16
	FieldSizeBits      uint16 // very important for reading bitpacked fields; size is the sum of all array pieces in bits - for example, uint32[3] will appear here as '96'
	AdditionalDataSize uint32 // the size in bytes of the corresponding section in common_data or pallet_data. These sections are in the same order as the field_info, so to find the offset, add up the additional_data_size of any previous fields which are stored in the same block
	StorageType        FieldCompression
	// Additional fields depend on storage_type
	StorageData struct {
		Bitpacked struct {
			OffsetBits uint32
			SizeBits   uint32
			Flags      uint32
		}
		BitpackedIndexed struct {
			OffsetBits uint32
			SizeBits   uint32
			ArrayCount uint32
		}
		CommonData struct {
			Default uint32
		}
		Unknown struct {
			D1 uint32
			D2 uint32
			D3 uint32
		}
	}
}

// Holds encryption status of specific ID in a section where tact_key_hash is not 0
type EncryptedStatus struct {
	EncryptedIDCount int32
	// Followed by: encrypted_id[encrypted_id_count]
}

// Copy table entry for deduplication
type CopyTableEntry struct {
	IDOfNewRow    uint32
	IDOfCopiedRow uint32
}

// Offset map entry
type OffsetMapEntry struct {
	Offset uint32
	Size   uint16
}

// Relationship entry
type RelationshipEntry struct {
	ForeignID   uint32 // This is the id of the foreign key for the record, e.g. SpellID in SpellX* tables.
	RecordIndex uint32 // This is the index of the record in record_data. Note that this is *not* the record's own ID *unless* flag 0x02 is set.
}

// Relationship mapping
type RelationshipMapping struct {
	NumEntries uint32
	MinID      uint32
	MaxID      uint32
	// Followed by: entries[num_entries]
}

// Common data map entry
type CommonDataMapEntry struct {
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

// FileReader interface for reading from files or CASC
type FileReader interface {
	Seek(offset int64, whence int) (int64, error)
	Read(p []byte) (n int, err error)
	ReadAt(p []byte, off int64) (n int, err error)
}

// File represents a WDC5 DB2 file
type File struct {
	reader            FileReader
	header            Header
	sections          []Section
	fieldStorageInfos []FieldStorageInfo
	palletData        []uint32
	commonData        map[uint32]uint32
	cache             []byte
}

// Section represents a section in the DB2 file
type Section struct {
	file              *File
	header            SectionHeader
	recordRegion      []byte
	stringRegion      []byte
	copyTable         map[uint32]uint32
	foreignKeyMap     map[uint32]uint32
	idList            []uint32
	recordSectionSize uint32
}

// Open opens a WDC5 DB2 file from a reader
func Open(reader FileReader) (*File, error) {
	file := &File{reader: reader}

	// Read header
	if err := binary.Read(reader, binary.LittleEndian, &file.header); err != nil {
		return nil, err
	}

	// Verify magic
	if string(file.header.Magic[:]) != "WDC5" {
		return nil, errors.New("invalid WDC5 magic")
	}

	// Read field storage infos
	file.fieldStorageInfos = make([]FieldStorageInfo, file.header.FieldCount)
	for i := range file.fieldStorageInfos {
		if err := binary.Read(reader, binary.LittleEndian, &file.fieldStorageInfos[i]); err != nil {
			return nil, err
		}
		// Read additional storage data based on type
		switch file.fieldStorageInfos[i].StorageType {
		case FieldCompressionBitpacked:
			binary.Read(reader, binary.LittleEndian, &file.fieldStorageInfos[i].StorageData.Bitpacked)
		case FieldCompressionBitpackedIndexed, FieldCompressionBitpackedIndexedArray:
			binary.Read(reader, binary.LittleEndian, &file.fieldStorageInfos[i].StorageData.BitpackedIndexed)
		case FieldCompressionCommonData:
			binary.Read(reader, binary.LittleEndian, &file.fieldStorageInfos[i].StorageData.CommonData)
		default:
			binary.Read(reader, binary.LittleEndian, &file.fieldStorageInfos[i].StorageData.Unknown)
		}
	}

	// Read pallet data
	file.palletData = make([]uint32, file.header.PalletDataSize/4)
	if err := binary.Read(reader, binary.LittleEndian, &file.palletData); err != nil {
		return nil, err
	}

	// Read common data
	file.commonData = make(map[uint32]uint32)
	commonDataBytes := make([]byte, file.header.CommonDataSize)
	if _, err := reader.Read(commonDataBytes); err != nil {
		return nil, err
	}
	// Parse common data entries
	offset := 0
	for offset < len(commonDataBytes) {
		var entry CommonDataMapEntry
		if err := binary.Read(io.NewSectionReader(reader, int64(offset), int64(len(commonDataBytes)-offset)), binary.LittleEndian, &entry); err != nil {
			break
		}
		file.commonData[entry.ID] = entry.RawValue
		offset += int(unsafe.Sizeof(entry))
	}

	// Read section headers
	sectionHeaders := make([]SectionHeader, file.header.SectionCount)
	for i := range sectionHeaders {
		if err := binary.Read(reader, binary.LittleEndian, &sectionHeaders[i]); err != nil {
			return nil, err
		}
	}

	// Initialize sections
	file.sections = make([]Section, len(sectionHeaders))
	for i, hdr := range sectionHeaders {
		section, err := file.initSection(hdr)
		if err != nil {
			return nil, err
		}
		file.sections[i] = *section
	}

	return file, nil
}

// initSection initializes a section
func (f *File) initSection(header SectionHeader) (*Section, error) {
	section := &Section{
		file:   f,
		header: header,
	}

	// Seek to section offset
	if _, err := f.reader.Seek(int64(header.FileOffset), io.SeekStart); err != nil {
		return nil, err
	}

	// Read the entire section into cache
	sectionSize := header.OffsetRecordsEnd - header.FileOffset
	cache := make([]byte, sectionSize)
	if _, err := f.reader.Read(cache); err != nil {
		return nil, err
	}

	// Parse section data
	recordRegion := cache[:header.OffsetRecordsEnd-header.FileOffset-header.StringTableSize]
	stringRegion := cache[len(recordRegion):]

	section.recordRegion = recordRegion
	section.stringRegion = stringRegion
	section.recordSectionSize = uint32(len(recordRegion))

	// Load copy table
	copyTableOffset := header.IDListSize
	copyTableEntries := make([]CopyTableEntry, header.CopyTableCount)
	for i := range copyTableEntries {
		offset := copyTableOffset + uint32(i)*uint32(unsafe.Sizeof(CopyTableEntry{}))
		if err := binary.Read(io.NewSectionReader(f.reader, int64(header.FileOffset+offset), int64(len(cache)-int(offset))), binary.LittleEndian, &copyTableEntries[i]); err != nil {
			return nil, err
		}
	}
	section.copyTable = make(map[uint32]uint32)
	for _, entry := range copyTableEntries {
		section.copyTable[entry.IDOfNewRow] = entry.IDOfCopiedRow
	}

	// Load relationship data
	if header.RelationshipDataSize > 0 {
		relOffset := header.IDListSize + header.CopyTableCount*uint32(unsafe.Sizeof(CopyTableEntry{})) + header.OffsetMapIDCount*uint32(unsafe.Sizeof(OffsetMapEntry{}))
		if f.hasRelationshipData() {
			relOffset += header.OffsetMapIDCount * 4
		}
		var relMapping RelationshipMapping
		if err := binary.Read(io.NewSectionReader(f.reader, int64(header.FileOffset+relOffset), int64(len(cache)-int(relOffset))), binary.LittleEndian, &relMapping); err != nil {
			return nil, err
		}
		relOffset += uint32(unsafe.Sizeof(relMapping))
		relEntries := make([]RelationshipEntry, relMapping.NumEntries)
		for i := range relEntries {
			offset := relOffset + uint32(i)*uint32(unsafe.Sizeof(RelationshipEntry{}))
			if err := binary.Read(io.NewSectionReader(f.reader, int64(header.FileOffset+offset), int64(len(cache)-int(offset))), binary.LittleEndian, &relEntries[i]); err != nil {
				return nil, err
			}
		}
		section.foreignKeyMap = make(map[uint32]uint32)
		for _, entry := range relEntries {
			section.foreignKeyMap[entry.ForeignID] = entry.RecordIndex
		}
	}

	// Get ID list
	idListSize := header.IDListSize / 4
	section.idList = make([]uint32, idListSize)
	for i := range section.idList {
		offset := uint32(i) * 4
		if err := binary.Read(io.NewSectionReader(f.reader, int64(header.FileOffset+offset), int64(len(cache)-int(offset))), binary.LittleEndian, &section.idList[i]); err != nil {
			return nil, err
		}
	}

	return section, nil
}

// Close closes the file
func (f *File) Close() error {
	// No-op for now, as we don't hold file handles
	return nil
}

// Records returns an iterator over all records in the file
func (f *File) Records() *FileRecordIter {
	return &FileRecordIter{
		file:         f,
		sectionIndex: 0,
		recordIndex:  0,
	}
}

// FieldStorageInfos returns the field storage infos
func (f *File) FieldStorageInfos() []FieldStorageInfo {
	return f.fieldStorageInfos
}

// HasRelationshipData checks if the file has relationship data
func (f *File) HasRelationshipData() bool {
	return (f.header.Flags & FlagHasRelationshipData) != 0
}

// HasNonInlineIDs checks if the file has non-inline IDs
func (f *File) HasNonInlineIDs() bool {
	return (f.header.Flags & FlagHasNonInlineIDs) != 0
}

// FileRecordIter iterates over all records in the file
type FileRecordIter struct {
	file         *File
	sectionIndex int
	recordIndex  int
}

// Next returns the next record
func (it *FileRecordIter) Next() *FixedRecord {
	for it.sectionIndex < len(it.file.sections) {
		section := &it.file.sections[it.sectionIndex]
		if it.recordIndex < int(section.header.RecordCount) {
			record := section.getRecord(it.recordIndex)
			it.recordIndex++
			return record
		}
		it.sectionIndex++
		it.recordIndex = 0
	}
	return nil
}

// getRecord gets a record from the section
func (s *Section) getRecord(index int) *FixedRecord {
	recordSize := s.file.header.RecordSize
	var id *uint32
	if s.file.HasNonInlineIDs() {
		id = &s.idList[index]
	}
	regionIndex := index * int(recordSize)
	fieldData := s.recordRegion[regionIndex : regionIndex+int(recordSize)]
	return &FixedRecord{
		id:      id,
		fields:  s.file.FieldStorageInfos(),
		data:    NewBitBuffer(fieldData),
		section: s,
		index:   regionIndex,
	}
}

// FixedRecord represents a fixed record
type FixedRecord struct {
	id      *uint32
	fields  []FieldStorageInfo
	data    *BitBuffer
	section *Section
	index   int
}

// GetID returns the ID of the record
func (r *FixedRecord) GetID() uint32 {
	if r.id != nil {
		return *r.id
	}
	idField := r.GetField(int(r.section.file.header.IDIndex))
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
func (r *FixedRecord) NumFields() int {
	if r.id == nil {
		return len(r.fields) - 1
	}
	return len(r.fields)
}

// GetField returns the field at the given index
func (r *FixedRecord) GetField(index int) interface{} {
	if r.id != nil {
		return r.getFieldWithID(index)
	}
	if index < int(r.section.file.header.IDIndex) {
		return r.getFieldWithID(index)
	}
	return r.getFieldWithID(index + 1)
}

// GetFieldAsString returns the field as a string
func (r *FixedRecord) GetFieldAsString(index int) string {
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
	case []uint32:
		stringIndex = f[0]
	}

	if stringIndex == 0 {
		return ""
	}
	// String indexes are referenced from where the field begins in the record.
	// Calculate the index from the beginning of the record.
	recordIndex := int(stringIndex) + r.index + int(r.fields[index].FieldOffsetBits/8)
	// get_string indexes from the string block
	return r.section.getString(recordIndex - len(r.section.recordRegion))
}

// getFieldWithID gets the field with the given ID
func (r *FixedRecord) getFieldWithID(index int) interface{} {
	field := r.fields[index]
	switch field.StorageType {
	case FieldCompressionNone:
		offset := field.FieldOffsetBits / 8
		size := field.FieldSizeBits / 8
		return r.data.inner[offset : offset+size]
	case FieldCompressionBitpacked:
		return r.data.GetUnsigned(int(field.FieldOffsetBits), int(field.FieldSizeBits))
	case FieldCompressionBitpackedSigned:
		return r.data.GetSigned(int(field.FieldOffsetBits), int(field.FieldSizeBits))
	case FieldCompressionBitpackedIndexed:
		palletIndex := r.data.GetUnsigned(int(field.FieldOffsetBits), int(field.FieldSizeBits))
		return r.section.file.palletData[palletIndex]
	case FieldCompressionBitpackedIndexedArray:
		palletIndex := r.data.GetUnsigned(int(field.FieldOffsetBits), int(field.FieldSizeBits))
		arrayCount := field.StorageData.BitpackedIndexed.ArrayCount
		return r.section.file.palletData[palletIndex : palletIndex+uint64(arrayCount)]
	case FieldCompressionCommonData:
		if val, ok := r.section.file.commonData[r.GetID()]; ok {
			return val
		}
		return field.StorageData.CommonData.Default
	}
	return nil
}

// getString gets a string from the string region
func (s *Section) getString(index int) string {
	if index < 0 || index >= len(s.stringRegion) {
		return ""
	}
	// Find null terminator
	for i := index; i < len(s.stringRegion); i++ {
		if s.stringRegion[i] == 0 {
			return string(s.stringRegion[index:i])
		}
	}
	return string(s.stringRegion[index:])
}

// BitBuffer for bit-level operations
type BitBuffer struct {
	inner []byte
}

// NewBitBuffer creates a new BitBuffer
func NewBitBuffer(buffer []byte) *BitBuffer {
	return &BitBuffer{inner: buffer}
}

// GetUnsigned gets an unsigned value from the buffer
func (b *BitBuffer) GetUnsigned(index, size int) uint64 {
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
func (b *BitBuffer) GetSigned(index, size int) int64 {
	value := b.GetUnsigned(index, size)
	// Do bit extend if needed
	signMask := uint64(1) << uint(size-1)
	if (value & signMask) != 0 {
		value |= ^((uint64(1) << uint(size)) - 1)
	}
	return int64(value)
}
