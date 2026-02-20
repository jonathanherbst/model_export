const std = @import("std");

const Error = error{
    UnknownColumn,
    InvalidIntegerSize,
};

const FieldType = enum {
    U8,
    S8,
    U16,
    S16,
    U32,
    S32,
    U64,
    S64,
    Float,
    String,
    LocString,

    pub fn from_parts(cdft: ColumnDefFieldType, size: ?FieldSize) !FieldType {
        switch (cdft) {
            .int => {
                if (size) |s| {
                    switch (s.size) {
                        8 => if (s.unsigned) return FieldType.U8 else return FieldType.S8,
                        16 => if (s.unsigned) return FieldType.U16 else return FieldType.S16,
                        32 => if (s.unsigned) return FieldType.U32 else return FieldType.S32,
                        64 => if (s.unsigned) return FieldType.U64 else return FieldType.S64,
                        else => return Error.InvalidIntegerSize,
                    }
                }
                return FieldType.S32;
            },
            .float => return FieldType.Float,
            .string => return FieldType.String,
            .locstring => return FieldType.LocString,
        }
    }

    pub fn is_string(self: @This()) bool {
        switch (self) {
            .LocString => return true,
            .String => return true,
            else => return false,
        }
    }
};

pub const Column = struct {
    name: []const u8,
    field_type: FieldType,
    annotations: Annotations,
    array: ?usize,
};

pub const SchemaSelector = union(enum) {
    build: []const u8,
    layout: []const u8,

    pub fn to_build_version(self: @This()) ?BuildVersion {
        switch (self) {
            .build => |build_str| {
                return BuildVersion.from_string(build_str);
            },
            else => {
                return null;
            },
        }
    }
};

pub const DBD = struct {
    id_index: usize,
    columns: std.array_list.AlignedManaged(Column, null),
    allocator: std.mem.Allocator,

    pub fn from_reader(path: []const u8, selector: SchemaSelector, allocator: std.mem.Allocator) !DBD {
        const maybe_build_ver = selector.to_build_version();
        const stat = try std.fs.cwd().statFile(path);
        const data = try std.fs.cwd().readFileAlloc(allocator, path, stat.size);
        errdefer allocator.free(data);

        var column_defs = std.StringHashMap(ColumnDef).init(allocator);
        defer column_defs.deinit();

        var columns = std.array_list.Managed(Column).init(allocator);
        errdefer columns.deinit();

        var lines = std.mem.splitScalar(u8, data, '\n');
        var state: enum { column_defs, builds, build_select, nothing } = .nothing;
        var id_index: usize = 0;
        while (lines.next()) |line_raw| {
            const line = std.mem.trim(u8, line_raw, " \t\r\n");
            if (line.len == 0) {
                if (state == .build_select) {
                    break;
                } else {
                    state = .nothing;
                }
            } else if (std.mem.eql(u8, line, "COLUMNS")) {
                state = .column_defs;
            } else if (std.mem.startsWith(u8, line, "COMMENT")) {
                continue;
            } else if (std.mem.startsWith(u8, line, "LAYOUT ")) {
                state = .builds;
                switch (selector) {
                    .layout => |selected_hash| {
                        const layout_hash = std.mem.trim(u8, line[7..], " \t");
                        if (std.mem.eql(u8, selected_hash, layout_hash)) {
                            state = .build_select;
                        }
                    },
                    else => {},
                }
            } else if (std.mem.startsWith(u8, line, "BUILD ")) {
                if (state != .build_select) {
                    state = .builds;
                    if (maybe_build_ver) |build_ver| {
                        var build_it = std.mem.splitScalar(u8, line[6..], ',');
                        while (build_it.next()) |build_str| {
                            if (build_ver.eql(BuildVersion.from_string(build_str))) {
                                state = .build_select;
                            }
                        }
                    }
                }
            } else if (state == .column_defs) {
                if (ColumnDef.from_string(line)) |column_def| {
                    try column_defs.put(column_def.name, column_def);
                }
            } else if (state == .build_select) {
                if (RecordColumnDef.from_string(line)) |record_column_def| {
                    // build the record here
                    if (column_defs.get(record_column_def.column_name)) |column_def| {
                        try columns.append(.{
                            .name = try allocator.dupe(u8, record_column_def.column_name),
                            .field_type = try FieldType.from_parts(column_def.field_type, record_column_def.size),
                            .annotations = record_column_def.annotations,
                            .array = record_column_def.array_length,
                        });
                        if (columns.getLast().annotations.id) {
                            id_index = columns.items.len - 1;
                        }
                    } else {
                        return Error.UnknownColumn;
                    }
                }
            }
        }

        return .{
            .id_index = id_index,
            .columns = columns,
            .allocator = allocator,
        };
    }

    pub fn deinit(self: @This()) void {
        for (self.columns.items) |column| {
            self.allocator.free(column.name);
        }
        self.columns.deinit();
    }

    pub fn num_columns(self: @This()) usize {
        return self.columns.items.len - 1;
    }

    pub fn get_column(self: @This(), index: usize) Column {
        if (index < self.id_index) {
            return self.columns.items[index];
        } else {
            return self.columns.items[index + 1];
        }
    }
};

const Annotations = struct { id: bool, noninline: bool, relation: bool };
const FieldSize = struct { size: usize, unsigned: bool };

const RecordColumnDef = struct {
    column_name: []const u8,
    annotations: Annotations,
    size: ?FieldSize,
    array_length: ?usize,

    pub fn from_string(str: []const u8) ?RecordColumnDef {
        // look for annotations
        var annotations = Annotations{ .id = false, .noninline = false, .relation = false };
        var remaining = str;
        if (std.mem.startsWith(u8, remaining, "$")) {
            var it = std.mem.splitScalar(u8, remaining[1..], '$');
            var annotation_it = std.mem.splitScalar(u8, it.next() orelse "", ',');
            while (annotation_it.next()) |annotation_str| {
                if (std.meta.stringToEnum(enum { id, noninline, relation }, annotation_str)) |annotation| {
                    switch (annotation) {
                        .id => annotations.id = true,
                        .noninline => annotations.noninline = true,
                        .relation => annotations.relation = true,
                    }
                }
            }

            if (it.next()) |rest| {
                remaining = rest;
            } else {
                return null;
            }
        }

        // filter out comment
        var comment_split = std.mem.splitAny(u8, remaining, " /");
        const rest = comment_split.next() orelse "";

        // find array length
        var array_length: ?usize = null;
        var length_split = std.mem.splitScalar(u8, rest, '[');
        const name_size = length_split.next() orelse "";
        if (length_split.next()) |array_length_str| {
            array_length = std.fmt.parseInt(usize, array_length_str[0 .. array_length_str.len - 1], 10) catch return null;
        }

        // find the field size
        var size: ?FieldSize = null;
        var size_split = std.mem.splitScalar(u8, name_size, '<');
        const name = size_split.next() orelse "";
        if (size_split.next()) |size_str| {
            if (std.mem.startsWith(u8, size_str, "u")) {
                const value = std.fmt.parseInt(usize, size_str[1 .. size_str.len - 1], 10) catch return null;
                size = FieldSize{ .size = value, .unsigned = true };
            } else {
                const value = std.fmt.parseInt(usize, size_str[0 .. size_str.len - 1], 10) catch return null;
                size = FieldSize{ .size = value, .unsigned = false };
            }
        }

        if (name.len > 0) {
            return RecordColumnDef{ .column_name = name, .annotations = annotations, .size = size, .array_length = array_length };
        } else {
            return null;
        }
    }
};

const ColumnDefFieldType = enum {
    int,
    float,
    string,
    locstring,
};

const ColumnDef = struct {
    name: []const u8,
    field_type: ColumnDefFieldType,
    foreign_key: ?[]const u8,
    verified: bool,

    pub fn from_string(str: []const u8) ?ColumnDef {
        var it = std.mem.splitScalar(u8, str, ' ');

        const field_type_str = it.next() orelse "";
        var field_type_it = std.mem.splitScalar(u8, field_type_str, '<');
        var field_type: ColumnDefFieldType = undefined;
        if (std.meta.stringToEnum(ColumnDefFieldType, field_type_it.next() orelse "")) |value| {
            field_type = value;
        } else {
            return null;
        }
        var foreign_key: ?[]const u8 = null;
        if (field_type_it.next()) |fk_str| {
            foreign_key = std.mem.trimEnd(u8, fk_str, &[_]u8{'>'});
        }

        var name: []const u8 = undefined;
        var verified = true;
        if (it.next()) |name_str| {
            if (std.mem.endsWith(u8, name_str, "?")) {
                verified = false;
                name = name_str[0 .. name_str.len - 1];
            } else {
                name = name_str;
            }
        } else {
            return null;
        }

        return ColumnDef{
            .name = name,
            .field_type = field_type,
            .foreign_key = foreign_key,
            .verified = verified,
        };
    }
};

const BuildVersion = struct {
    lower: u64,
    upper: u64,

    pub fn from_string(str: []const u8) BuildVersion {
        var it = std.mem.splitScalar(u8, str, '-');
        const lower = BuildVersion.parse_version(it.next() orelse "");
        const upper = BuildVersion.parse_version(it.next() orelse "");
        if (upper < lower) {
            return BuildVersion{ .lower = lower, .upper = lower };
        } else {
            return BuildVersion{ .lower = lower, .upper = upper };
        }
    }

    fn parse_version(str: []const u8) u64 {
        var it = std.mem.splitScalar(u8, str, '.');
        const major = std.fmt.parseUnsigned(u64, it.next() orelse "", 10) catch 0;
        const minor = std.fmt.parseUnsigned(u64, it.next() orelse "", 10) catch 0;
        const patch = std.fmt.parseUnsigned(u64, it.next() orelse "", 10) catch 0;
        const revision = std.fmt.parseUnsigned(u64, it.next() orelse "", 10) catch 0;

        return major << 48 | minor << 40 | patch << 32 | revision;
    }

    pub fn eql(self: BuildVersion, other: BuildVersion) bool {
        return self.lower <= other.lower and self.upper >= other.upper;
    }
};

const expect = std.testing.expect;

test "build version" {
    try expect(BuildVersion.parse_version("3.4.1.8622") == 848827271618990);
    try expect(BuildVersion.parse_version("3.4.1.8622.") == 848827271618990);
    try expect(BuildVersion.parse_version("3.4.1.") == 848827271610368);
    try expect(BuildVersion.parse_version("3.4.1.q") == 848827271610368);
    try expect(BuildVersion.parse_version("3.4.1") == 848827271610368);
    try expect(BuildVersion.parse_version("asdfsd") == 0);

    const version = BuildVersion.from_string("3.4.1.8622-3.4.7.2329");
    try expect(version.eql(version));
    try expect(!version.eql(BuildVersion.from_string("3.4.1.8621")));
    try expect(!version.eql(BuildVersion.from_string("3.4.7.2330")));
    try expect(version.eql(BuildVersion.from_string("3.4.1.8622")));
    try expect(version.eql(BuildVersion.from_string("3.4.6.8622")));
    try expect(version.eql(BuildVersion.from_string("3.4.7.2329")));
}

test "column definition" {
    const col = ColumnDef.from_string("int ID").?;
    try expect(std.mem.eql(u8, col.name, "ID"));
    try expect(col.field_type == FieldType.int);
    try expect(col.foreign_key == null);
    try expect(col.verified);

    const col2 = ColumnDef.from_string("locstring MapDescription0_lang // Horde").?;
    try expect(std.mem.eql(u8, col2.name, "MapDescription0_lang"));
    try expect(col2.field_type == FieldType.locstring);
    try expect(col2.foreign_key == null);
    try expect(col2.verified);

    const col3 = ColumnDef.from_string("int<FileData::ID> WdtFileDataID?").?;
    try expect(std.mem.eql(u8, col3.name, "WdtFileDataID"));
    try expect(col3.field_type == FieldType.int);
    try expect(std.mem.eql(u8, col3.foreign_key.?, "FileData::ID"));
    try expect(!col3.verified);

    const col4 = ColumnDef.from_string("string InternalName").?;
    try expect(std.mem.eql(u8, col4.name, "InternalName"));
    try expect(col4.field_type == FieldType.string);
    try expect(col4.foreign_key == null);
    try expect(col4.verified);

    const col5 = ColumnDef.from_string("float Field_0_7_0_3694_007?").?;
    try expect(std.mem.eql(u8, col5.name, "Field_0_7_0_3694_007"));
    try expect(col5.field_type == FieldType.float);
    try expect(col5.foreign_key == null);
    try expect(!col5.verified);
}

test "column" {
    var col = RecordColumnDef.from_string("$noninline,id$ID<32>").?;
    try expect(std.mem.eql(u8, col.column_name, "ID"));
    try expect(col.annotations.id == true);
    try expect(col.annotations.noninline == true);
    try expect(col.annotations.relation == false);
    try expect(col.array_length == null);
    try expect(col.size.?.size == 32);
    try expect(col.size.?.unsigned == false);

    col = RecordColumnDef.from_string("Flags<32>[3]").?;
    try expect(std.mem.eql(u8, col.column_name, "Flags"));
    try expect(col.annotations.id == false);
    try expect(col.annotations.noninline == false);
    try expect(col.annotations.relation == false);
    try expect(col.array_length.? == 3);
    try expect(col.size.?.size == 32);
    try expect(col.size.?.unsigned == false);

    col = RecordColumnDef.from_string("ExpansionID<u8>").?;
    try expect(std.mem.eql(u8, col.column_name, "ExpansionID"));
    try expect(col.annotations.id == false);
    try expect(col.annotations.noninline == false);
    try expect(col.annotations.relation == false);
    try expect(col.array_length == null);
    try expect(col.size.?.size == 8);
    try expect(col.size.?.unsigned == true);

    col = RecordColumnDef.from_string("$id$ID<32>").?;
    try expect(std.mem.eql(u8, col.column_name, "ID"));
    try expect(col.annotations.id == true);
    try expect(col.annotations.noninline == false);
    try expect(col.annotations.relation == false);
    try expect(col.array_length == null);
    try expect(col.size.?.size == 32);
    try expect(col.size.?.unsigned == false);

    col = RecordColumnDef.from_string("Corpse[2] // a comment").?;
    try expect(std.mem.eql(u8, col.column_name, "Corpse"));
    try expect(col.annotations.id == false);
    try expect(col.annotations.noninline == false);
    try expect(col.annotations.relation == false);
    try expect(col.array_length.? == 2);
    try expect(col.size == null);
}
