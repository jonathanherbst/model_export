const std = @import("std");

const dbd = @import("dbd.zig");
const wdc5 = @import("wdc5.zig");

pub const DefinedRecord = struct {
    schema: dbd.DBD,
    record: wdc5.FixedRecord,

    pub fn num_fields(self: @This()) usize {
        std.debug.assert(self.record.num_fields() == self.schema.num_columns());
        return self.record.num_fields();
    }

    pub fn get_field(self: @This(), index: usize) DefinedField {
        return .{
            .index = index,
            .column = self.schema.get_column(index),
            .field = self.record.get_field(index),
            .record = self.record,
        };
    }
};

const Value = union(enum) {
    signed: i64,
    unsigned: u64,
    float: f32,
    string: []const u8,
};

const DefinedField = struct {
    index: usize,
    column: dbd.Column,
    field: wdc5.Field,
    record: wdc5.FixedRecord,

    pub fn num_values(self: @This()) usize {
        return self.column.array orelse 1;
    }

    pub fn get_value(self: @This(), index: usize) Value {
        if (self.column.field_type == .String or self.column.field_type == .LocString) {
            std.debug.assert(self.num_values() == 1);
            return .{ .string = std.mem.span(self.record.get_field_as_string(self.index)) };
        }
        switch (self.field) {
            .bytes => |v| {
                switch (self.column.field_type) {
                    .U8 => return .{ .unsigned = v[0] },
                    .S8 => return .{ .signed = @as(i8, @bitCast(v[0])) },
                    .U16 => return .{ .unsigned = std.mem.readInt(u16, v[0..2], .little) },
                    .S16 => return .{ .signed = std.mem.readInt(i16, v[0..2], .little) },
                    .U32 => return .{ .unsigned = std.mem.readInt(u32, v[0..4], .little) },
                    .S32 => return .{ .signed = std.mem.readInt(i32, v[0..4], .little) },
                    .U64 => return .{ .unsigned = std.mem.readInt(u64, v[0..8], .little) },
                    .S64 => return .{ .signed = std.mem.readInt(i64, v[0..8], .little) },
                    .Float => return .{ .float = @bitCast(std.mem.readInt(u32, v[0..4], .little)) },
                    .String, .LocString => unreachable,
                }
            },
            .signed => |v| {
                return .{ .signed = v };
            },
            .unsigned => |v| {
                switch (self.column.field_type) {
                    .Float => return .{ .float = @bitCast(@as(u32, @intCast(v))) },
                    .String, .LocString => unreachable,
                    else => return .{ .unsigned = v },
                }
            },
            .indexed => |v| {
                switch (self.column.field_type) {
                    .U8, .U16, .U32, .U64 => return .{ .unsigned = @intCast(v[index]) },
                    .S8, .S16, .S32, .S64 => return .{ .signed = @as(i32, @bitCast(v[index])) },
                    .Float => return .{ .float = @as(f32, @bitCast(v[index])) },
                    .String, .LocString => unreachable,
                }
            },
        }
    }
};
