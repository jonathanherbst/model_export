const std = @import("std");

const casc = @import("casc.zig");

pub const Error = error{
    InvalidFileHeader,
    NotEnoughData,
};

/// Structure of a WDC5 DB2 file
///
/// Header
/// SectionHeader[Header.section_count]
/// FieldStructure[Header.total_field_count]
/// FieldStorageInfo[Header.field_storage_info_size/sizeof(FieldStorageInfo)]
/// char[Header.pallet_data_size]
/// char[Header.common_data_size]
/// EncryptedStatus[num_encrypted_sections] // section is encrypted if SectionHeader.tact_key_hash != 0
/// Section[Header.section_count] // section format depends on the header and section header
pub const File = struct {
    file: FileReader,
    header: Header,
    cache: []const u8,
    common_data: std.AutoHashMap(u32, u32),
    pallet_data: []const u32,
    allocator: std.mem.Allocator,

    pub fn open(file: FileReader, allocator: std.mem.Allocator) !@This() {
        std.debug.assert(@sizeOf(Header) == 204);
        std.debug.assert(@sizeOf(SectionHeader) == 40);
        std.debug.assert(@sizeOf(FieldStructure) == 4);
        std.debug.assert(@sizeOf(FieldStorageInfo) == 24);
        try file.seekTo(0);
        const header = try file.readSized(Header);
        // make sure the file is what we can parse here.
        if (!std.mem.eql(u8, &std.mem.toBytes(header.magic), "WDC5") or header.version != 5) {
            return Error.InvalidFileHeader;
        }

        // don't know how to deal with variable record data yet
        std.debug.assert((header.flags & 0x01 == 0));
        // don't know what to do if some fields aren't defined by field storage info yet
        std.debug.assert(header.field_count == header.field_storage_info_size / @sizeOf(FieldStorageInfo));

        // cache all the data before the records start
        const cache_size = header.section_count * @sizeOf(SectionHeader) +
            header.total_field_count * @sizeOf(FieldStructure) +
            header.field_storage_info_size + header.pallet_data_size + header.common_data_size;
        const cache = try allocator.alignedAlloc(u8, std.mem.Alignment.@"4", cache_size);
        errdefer allocator.free(cache);
        _ = try file.read(cache);

        // load the common data into a map
        const common_data_offset = header.section_count * @sizeOf(SectionHeader) +
            header.total_field_count * @sizeOf(FieldStructure) +
            header.field_storage_info_size + header.pallet_data_size;
        const common_data_arr: []const CommonDataMapEntry = @ptrCast(@alignCast(cache[common_data_offset .. common_data_offset + header.common_data_size]));
        var common_data = std.AutoHashMap(u32, u32).init(allocator);
        for (common_data_arr) |entry| {
            try common_data.put(entry.id, entry.raw_value);
        }

        // setup the slice for the pallet data
        const offset = header.section_count * @sizeOf(SectionHeader) +
            header.total_field_count * @sizeOf(FieldStructure) +
            header.field_storage_info_size;
        const pallet_data: []const u32 = @ptrCast(@alignCast(cache[offset .. offset + header.pallet_data_size]));

        return .{
            .file = file,
            .header = header,
            .cache = cache,
            .common_data = common_data,
            .pallet_data = pallet_data,
            .allocator = allocator,
        };
    }

    pub fn close(self: *@This()) void {
        self.allocator.free(self.cache);
        self.common_data.deinit();
    }

    pub fn get_schema_str(self: *const File) []const u8 {
        const schema_str: [*:0]const u8 = @ptrCast(&self.header.schema_string);
        return std.mem.span(schema_str);
    }

    pub fn get_layout_hash(self: File) u32 {
        return self.header.layout_hash;
    }

    pub fn has_variable_records(self: @This()) bool {
        return self.header.flags & 0x01 != 0;
    }

    pub fn has_relationship_data(self: @This()) bool {
        return self.header.flags & 0x02 != 0;
    }

    pub fn has_noninline_ids(self: @This()) bool {
        return self.header.flags & 0x04 != 0;
    }

    pub fn sections(self: *File) !SectionIter {
        return .{ .file = self, .section_headers = self.section_headers(), .index = 0, .allocator = self.allocator };
    }

    pub fn records(self: *File) !FileRecordIter {
        return .{
            .sections = try self.sections(),
            .section_records = null,
        };
    }

    pub fn section_headers(self: File) []const SectionHeader {
        return @ptrCast(@alignCast(self.cache[0 .. self.header.section_count * @sizeOf(SectionHeader)]));
    }

    pub fn field_structures(self: File) []const FieldStructure {
        const offset = self.header.section_count * @sizeOf(SectionHeader);
        return @ptrCast(@alignCast(self.cache[offset .. offset + self.total_field_count * @sizeOf(FieldStructure)]));
    }

    pub fn field_storage_infos(self: File) []const FieldStorageInfo {
        const offset = self.header.section_count * @sizeOf(SectionHeader) +
            self.header.total_field_count * @sizeOf(FieldStructure);
        return @ptrCast(@alignCast(self.cache[offset .. offset + self.header.field_storage_info_size]));
    }
};

/// Section Format
/// if ((header.flags & 1) == 0) {
///     char[Header.record_size][SectionHeader.record_count]
///     char[SectionHeader.string_table_size]
/// } else {
///     char[SectionHeader.offset_records_end - SectionHeader.file_offset]
/// }
/// u32[SectionHeader.id_list_size]
/// CopyTableEntry[SectionHeader.copy_table_count]
/// OffsetMapEntry[SectionHeader.offset_map_id_count]
/// if ((Header.flags & 0x02) != 0) {
///     u32[SectionHeader.offset_map_id_count]
/// }
/// if(SectionHeader.relationship_data_size > 0) {
///     u32 num_entries
///     u32 min_id
///     u32 max_id
///     RelationshipEntry[num_entries]
/// }
/// if ((Header.flags & 0x02) == 0) {
///     u32[SectionHeader.offset_map_id_count]
/// }
const Section = struct {
    file: *File,
    header: SectionHeader,
    record_region: []const u8,
    string_region: ?[]const u8,
    copy_table: std.hash_map.AutoHashMap(u32, u32),
    foreign_key_map: std.hash_map.AutoHashMap(u32, u32),
    id_list: []const u32,
    allocator: std.mem.Allocator,
    record_section_size: usize,

    pub fn init(file: *File, header: SectionHeader, allocator: std.mem.Allocator) !@This() {
        if (file.has_noninline_ids()) {
            std.debug.assert(header.record_count == header.id_list_size / 4);
        }

        // allocate cache for the entire section
        var record_section_size: usize = 0;
        if (!file.has_variable_records()) {
            record_section_size = file.header.record_size * header.record_count + header.string_table_size;
        } else {
            record_section_size = header.offset_records_end - header.file_offset;
        }
        var record_region = try allocator.alloc(u8, record_section_size);
        errdefer allocator.free(record_region);

        const end_section_size = header.id_list_size + header.copy_table_count * @sizeOf(CopyTableEntry) + header.offset_map_id_count * @sizeOf(OffsetMapEntry) + header.relationship_data_size + header.offset_map_id_count * 4;
        var end_section = try allocator.alignedAlloc(u8, std.mem.Alignment.@"4", end_section_size);
        defer allocator.free(end_section);

        // read the entire section from the file into the cache
        try file.file.seekTo(header.file_offset);
        _ = try file.file.read(record_region);
        _ = try file.file.read(end_section);

        // setup the record and string region slices
        var string_region: ?[]const u8 = null;
        if (!file.has_variable_records()) {
            const record_region_size = file.header.record_size * header.record_count;
            string_region = record_region[record_region_size .. record_region_size + header.string_table_size];
        }

        // load the copy table
        var copy_table = std.AutoHashMap(u32, u32).init(allocator);
        errdefer copy_table.deinit();
        try copy_table.ensureTotalCapacity(header.copy_table_count);
        const copy_table_offset = header.id_list_size;
        const copy_table_entries: []const CopyTableEntry = @ptrCast(@alignCast(end_section[copy_table_offset .. copy_table_offset + header.copy_table_count * @sizeOf(CopyTableEntry)]));
        for (copy_table_entries) |entry| {
            try copy_table.put(entry.id_of_new_row, entry.id_of_copied_row);
        }

        // load the relationship (foreign key) map
        var foreign_key_map = std.AutoHashMap(u32, u32).init(allocator);
        errdefer foreign_key_map.deinit();
        if (header.relationship_data_size > 0) {
            var rel_offset = header.id_list_size + header.copy_table_count * @sizeOf(CopyTableEntry) + header.offset_map_id_count * @sizeOf(OffsetMapEntry);
            if (file.has_relationship_data()) {
                rel_offset += header.offset_map_id_count * 4;
            }
            const rel_mapping = std.mem.bytesAsValue(RelationshipMapping, end_section[rel_offset .. rel_offset + @sizeOf(RelationshipMapping)]);
            rel_offset += @sizeOf(RelationshipMapping);
            const rel_entries: []const RelationshipEntry = @ptrCast(@alignCast(end_section[rel_offset .. rel_offset + @sizeOf(RelationshipEntry) * rel_mapping.num_entries]));
            try foreign_key_map.ensureTotalCapacity(rel_mapping.num_entries);
            for (rel_entries) |entry| {
                try foreign_key_map.put(entry.foreign_id, entry.record_index);
            }
        }

        // get a slice to the id list
        const cache_id_list: []const u32 = @ptrCast(@alignCast(end_section[0..header.id_list_size]));
        const id_list = try allocator.dupe(u32, cache_id_list);

        return .{
            .file = file,
            .header = header,
            .record_region = record_region,
            .string_region = string_region,
            .copy_table = copy_table,
            .foreign_key_map = foreign_key_map,
            .id_list = id_list,
            .allocator = allocator,
            .record_section_size = record_section_size,
        };
    }

    pub fn deinit(self: *@This()) void {
        self.foreign_key_map.deinit();
        self.copy_table.deinit();
        self.allocator.free(self.id_list);
        self.allocator.free(self.cache);
    }

    pub fn records(self: @This()) SectionRecordIter {
        return SectionRecordIter.all(self);
    }

    pub fn get_string(self: @This(), index: usize) [*:0]const u8 {
        return @ptrCast(&self.string_region.?[index]);
    }

    fn offset_map(self: @This()) []const OffsetMapEntry {
        const offset = self.record_section_size + self.header.id_list_size + self.header.copy_table_count * @sizeOf(CopyTableEntry);
        return @ptrCast(self.cache[offset .. offset + self.header.offset_map_id_count * @sizeOf(OffsetMapEntry)]);
    }

    fn offset_map_ids(self: @This()) []const u32 {
        var offset = self.record_section_size + self.header.id_list_size + self.header.copy_table_count * @sizeOf(CopyTableEntry) + self.header.offset_map_id_count * @sizeOf(OffsetMapEntry);
        if ((self.file.header.flags & 0x02) == 0) {
            // offset map id comes after relationship mapping
            offset += self.header.relationship_data_size;
        }
        return @ptrCast(self.cache[offset .. offset + self.header.offset_map_id_count * 4]);
    }
};

const FileRecordIter = struct {
    sections: SectionIter,
    section_records: ?SectionRecordIter,

    pub fn next(self: *@This()) ?FixedRecord {
        while (true) {
            if (self.section_records) |*records| {
                if (records.next()) |record| {
                    return record;
                }
            }
            // record iter is done
            if (self.sections.next()) |section| {
                self.section_records = section.records();
            } else {
                return null;
            }
        }
    }
};

const SectionRecordIter = struct {
    section: Section,
    index: usize,

    pub fn all(section: Section) @This() {
        std.debug.assert((section.file.header.flags & 0x01) == 0);
        return .{
            .section = section,
            .index = 0,
        };
    }

    pub fn next(self: *@This()) ?FixedRecord {
        if (self.section.header.tact_key_hash == 0) {
            const record_size = self.section.file.header.record_size;
            if (self.index < self.section.header.record_count) {
                var id: ?u32 = null;
                if (self.section.file.has_noninline_ids()) {
                    id = self.section.id_list[self.index];
                }
                const region_index = self.index * record_size;
                const field_data = self.section.record_region[region_index .. region_index + record_size];
                self.index += 1;
                return .{ .id = id, .fields = self.section.file.field_storage_infos(), .data = BitBuffer.from_buffer(field_data), .section = self.section, .index = region_index };
            } else {
                return null;
            }
        } else {
            // we don't know how to deal with encrypted sections yet so just return null here.
            return null;
        }
    }
};

pub const Field = union(enum) {
    bytes: []const u8,
    indexed: []const u32,
    signed: i64,
    unsigned: u64,
};

pub const FixedRecord = struct {
    id: ?u32,
    fields: []const FieldStorageInfo,
    data: BitBuffer,
    section: Section,
    index: usize,

    pub fn is_id_inline(self: @This()) bool {
        return self.id == null;
    }

    pub fn get_id(self: @This()) u32 {
        if (self.id) |id| {
            return id;
        } else {
            const id_field = self.get_field_with_id(self.section.file.header.id_index);
            switch (id_field) {
                .signed => |value| {
                    return @intCast(value);
                },
                .unsigned => |value| {
                    return @intCast(value);
                },
                else => std.debug.panic("id is not an integer type", .{}),
            }
        }
    }

    pub fn num_fields(self: @This()) usize {
        if (self.is_id_inline()) {
            return self.fields.len - 1;
        } else {
            return self.fields.len;
        }
    }

    pub fn get_field(self: @This(), index: usize) Field {
        if (self.id != null) {
            return self.get_field_with_id(index);
        } else {
            if (index < self.section.file.header.id_index) {
                return self.get_field_with_id(index);
            } else {
                return self.get_field_with_id(index + 1);
            }
        }
    }

    pub fn get_field_as_string(self: @This(), index: usize) [*:0]const u8 {
        var string_index: usize = 0;
        switch (self.get_field(index)) {
            .bytes => |value| {
                if (value.len == 4) {
                    string_index = std.mem.readInt(u32, value[0..4], .little);
                } else {
                    std.debug.panic("unknown conversion from bytes of len {} to string offset", .{value.len});
                }
            },
            .signed => |value| {
                string_index = @intCast(value);
            },
            .unsigned => |value| {
                string_index = value;
            },
            .indexed => |value| {
                string_index = value[0];
            },
        }

        if (string_index == 0) {
            return "";
        } else {
            // string indexes are referenced from the where the field begins in the record.
            // calculate the index from the beginning of the record.
            const record_index = string_index + self.index + self.fields[index].field_offset_bits / 8;
            // get_string indexes from the string block
            return self.section.get_string(record_index - self.section.record_region.len);
        }
    }

    fn get_field_with_id(self: @This(), index: usize) Field {
        const field = self.fields[index];
        switch (field.storage_type) {
            .field_compression_none => {
                const offset = field.field_offset_bits / 8;
                const size = field.field_size_bits / 8;
                return .{ .bytes = self.data.inner[offset .. offset + size] };
            },
            .field_compression_bitpacked => {
                return .{ .unsigned = self.data.get_unsigned(field.field_offset_bits, field.field_size_bits) };
            },
            .field_compression_bitpacked_signed => {
                return .{ .signed = self.data.get_signed(field.field_offset_bits, @intCast(field.field_size_bits)) };
            },
            .field_compression_bitpacked_indexed => {
                const pallet_index = self.data.get_unsigned(field.field_offset_bits, field.field_size_bits);
                return .{ .unsigned = self.section.file.pallet_data[pallet_index] };
            },
            .field_compression_bitpacked_indexed_array => {
                const pallet_index = self.data.get_unsigned(field.field_offset_bits, field.field_size_bits);
                return .{ .indexed = self.section.file.pallet_data[pallet_index .. pallet_index + field.storage_data.bitpacked_indexed.array_count] };
            },
            .field_compression_common_data => {
                return .{ .unsigned = self.section.file.common_data.get(self.get_id()) orelse field.storage_data.common_data.default };
            },
        }
    }
};

const CommonData = extern struct {
    record_id: u32,
    value: u32,
};

const SectionIter = struct {
    file: *File,
    section_headers: []const SectionHeader,
    index: usize,
    allocator: std.mem.Allocator,

    pub fn next(self: *@This()) ?Section {
        if (self.index < self.section_headers.len) {
            const hdr = self.section_headers[self.index];
            self.index += 1;
            return Section.init(self.file, hdr, self.allocator) catch return null;
        } else {
            return null;
        }
    }
};

pub const FileReader = struct {
    ptr: *anyopaque,
    seekToFn: *const fn (ptr: *anyopaque, offset: u64) anyerror!void,
    readFn: *const fn (ptr: *anyopaque, buffer: []u8) anyerror!usize,

    pub fn seekTo(self: @This(), offset: u64) !void {
        return try self.seekToFn(self.ptr, offset);
    }

    pub fn read(self: @This(), buffer: []u8) !usize {
        return try self.readFn(self.ptr, buffer);
    }

    pub fn readSized(self: @This(), comptime T: type) !T {
        var buffer: [@sizeOf(T)]u8 align(@alignOf(T)) = undefined;
        const bytes_read = try self.read(&buffer);
        if (bytes_read != @sizeOf(T)) {
            return Error.NotEnoughData;
        }
        return @bitCast(buffer);
    }

    pub fn from_file(file: *std.fs.File) @This() {
        const Wrapper = struct {
            fn seekTo(ptr: *anyopaque, offset: u64) anyerror!void {
                const self: *std.fs.File = @ptrCast(@alignCast(ptr));
                return try self.seekTo(offset);
            }

            fn read(ptr: *anyopaque, buffer: []u8) anyerror!usize {
                const self: *std.fs.File = @ptrCast(@alignCast(ptr));
                return try self.readAll(buffer);
            }
        };

        return .{
            .ptr = file,
            .seekToFn = Wrapper.seekTo,
            .readFn = Wrapper.read,
        };
    }

    pub fn from_casc_file(file: *casc.File) @This() {
        const Wrapper = struct {
            fn seekTo(ptr: *anyopaque, offset: u64) anyerror!void {
                const self: *casc.File = @ptrCast(@alignCast(ptr));
                _ = try self.seek(@intCast(offset));
            }

            fn read(ptr: *anyopaque, buffer: []u8) anyerror!usize {
                const self: *casc.File = @ptrCast(@alignCast(ptr));
                return try self.read(buffer);
            }
        };

        return .{
            .ptr = file,
            .seekToFn = Wrapper.seekTo,
            .readFn = Wrapper.read,
        };
    }
};

/// WDC5 DB2 Header
/// This section only applies to versions >= DF (10.2.5.52432)
const Header = extern struct {
    magic: [4]u8, // 'WDC5'
    version: u32, // 5, probably numeric version?
    schema_string: [128]u8, // "WowStatic_Patch_10_2_5" + padding in 10.2.5.52432
    record_count: u32, // this is for all sections combined now
    field_count: u32,
    record_size: u32,
    string_table_size: u32, // this is for all sections combined now
    table_hash: u32, // hash of the table name
    layout_hash: u32, // this is a hash field that changes only when the structure of the data changes
    min_id: u32,
    max_id: u32,
    locale: u32, // as seen in TextWowEnum
    flags: u16, // possible values are listed in Known Flag Meanings
    id_index: u16, // this is the index of the field containing ID values; this is ignored if flags & 0x04 != 0
    total_field_count: u32, // from WDC1 onwards, this value seems to always be the same as the 'field_count' value
    bitpacked_data_offset: u32, // relative position in record where bitpacked data begins; not important for parsing the file
    lookup_column_count: u32,
    field_storage_info_size: u32,
    common_data_size: u32,
    pallet_data_size: u32,
    section_count: u32, // new to WDC2, this is number of sections of data
};

/// WDC5 Section Header
pub const SectionHeader = extern struct {
    tact_key_hash: u64, // TactKeyLookup hash
    file_offset: u32, // absolute position to the beginning of the section
    record_count: u32, // 'record_count' for the section
    string_table_size: u32, // 'string_table_size' for the section
    offset_records_end: u32, // Offset to the spot where the records end in a file with an offset map structure;
    id_list_size: u32, // Size of the list of ids present in the section
    relationship_data_size: u32, // Size of the relationship data in the section
    offset_map_id_count: u32, // Count of ids present in the offset map in the section
    copy_table_count: u32, // Count of the number of deduplication entries
};

/// Field structure for WDC5
pub const FieldStructure = extern struct {
    size: i16, // size in bits as calculated by: byteSize = (32 - size) / 8; this value can be negative to indicate field sizes larger than 32-bits
    position: u16, // position of the field within the record, relative to the start of the record
};

/// Field compression types for WDC5
pub const FieldCompression = enum(i32) {
    /// None -- usually the field is a 8-, 16-, 32-, or 64-bit integer in the record data. But can contain 96-bit value representing 3 floats as well
    field_compression_none = 0,
    /// Bitpacked -- the field is a bitpacked integer in the record data.
    field_compression_bitpacked = 1,
    /// Common data -- the field is assumed to be a default value, and exceptions from that default value are stored in the corresponding section in common_data
    field_compression_common_data = 2,
    /// Bitpacked indexed -- the field has a bitpacked index in the record data.
    field_compression_bitpacked_indexed = 3,
    /// Bitpacked indexed array -- the field has a bitpacked index in the record data.
    field_compression_bitpacked_indexed_array = 4,
    /// Same as field_compression_bitpacked
    field_compression_bitpacked_signed = 5,
};

/// Field storage info for WDC5
pub const FieldStorageInfo = extern struct {
    field_offset_bits: u16,
    field_size_bits: u16, // very important for reading bitpacked fields; size is the sum of all array pieces in bits - for example, uint32[3] will appear here as '96'
    additional_data_size: u32, // the size in bytes of the corresponding section in common_data or pallet_data. These sections are in the same order as the field_info, so to find the offset, add up the additional_data_size of any previous fields which are stored in the same block
    storage_type: FieldCompression,
    // Additional fields depend on storage_type
    // This is a simplified version - the actual parsing would need to handle the union-like nature
    storage_data: extern union {
        bitpacked: extern struct { offset_bits: u32, size_bits: u32, flags: u32 },
        bitpacked_indexed: extern struct { offset_bits: u32, size_bits: u32, array_count: u32 },
        common_data: extern struct { default: u32 },
        unknown: extern struct { d1: u32, d2: u32, d3: u32 },
    },
};

/// Holds encryption status of specific ID in a section where tact_key_hash is not 0
pub const EncryptedStatus = extern struct {
    encrypted_id_count: i32,
    // Followed by: encrypted_id[encrypted_id_count]
};

/// Copy table entry for deduplication
pub const CopyTableEntry = extern struct {
    id_of_new_row: u32,
    id_of_copied_row: u32,
};

/// Offset map entry
pub const OffsetMapEntry = extern struct {
    offset: u32,
    size: u16,
};

/// Relationship entry
pub const RelationshipEntry = extern struct {
    foreign_id: u32, // This is the id of the foreign key for the record, e.g. SpellID in SpellX* tables.
    record_index: u32, // This is the index of the record in record_data. Note that this is *not* the record's own ID *unless* flag 0x02 is set.
};

/// Relationship mapping
pub const RelationshipMapping = extern struct {
    num_entries: u32,
    min_id: u32,
    max_id: u32,
    // Followed by: entries[num_entries]
};

/// Common data map entry
pub const CommonDataMapEntry = extern struct {
    id: u32,
    raw_value: u32,
};

/// Known flag meanings for WDC5
pub const Flags = struct {
    pub const HAS_OFFSET_MAP: u16 = 0x01; // 'Has offset map'
    pub const HAS_RELATIONSHIP_DATA: u16 = 0x02; // 'Has relationship data'
    pub const HAS_NON_INLINE_IDS: u16 = 0x04; // 'Has non-inline IDs'
    pub const IS_BITPACKED: u16 = 0x10; // 'Is bitpacked'
};

const BitBuffer = struct {
    inner: []const u8,

    pub fn from_buffer(buffer: []const u8) @This() {
        return .{
            .inner = buffer,
        };
    }

    pub fn get_unsigned(self: *const BitBuffer, index: usize, size: usize) u64 {
        std.debug.assert(size > 0 and size < 64);
        var data: [8]u8 align(8) = undefined;
        const size_bytes = (size + (index & 7) + 7) / 8;
        const index_bytes = index / 8;
        std.mem.copyForwards(u8, &data, self.inner[index_bytes .. index_bytes + size_bytes]);
        const data_u64: u64 = @bitCast(data);
        return (data_u64 >> @intCast(index & 7)) & ((@as(u64, 1) << @intCast(size)) - 1);
    }

    pub fn get_signed(self: *const BitBuffer, index: usize, size: u6) i64 {
        var value = self.get_unsigned(index, size);
        // do bit extend if it's needed
        const sign_mask = @as(u64, 1) << (size - 1);
        if ((value & sign_mask) != 0) {
            value |= ~((@as(u64, 1) << size) - 1);
        }
        return @bitCast(value);
    }
};

test "bitbuffer" {
    const expectEqual = std.testing.expectEqual;

    const data_int: u32 = 0x67452301;
    const buffer = BitBuffer.from_buffer(&std.mem.toBytes(data_int));
    try expectEqual(0, buffer.get_unsigned(1, 7));
    try expectEqual(0x23, buffer.get_unsigned(8, 8));
    try expectEqual(0x4523, buffer.get_unsigned(8, 16));
    try expectEqual(582, buffer.get_unsigned(7, 10));
}
