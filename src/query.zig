const std = @import("std");

pub const Error = error{
    InvalidSyntax,
};

pub const Select = struct {
    columns: []const Column,
    from: From,
};

pub const Column = struct {
    name: []const u8,
    table: ?[]const u8,
};

pub const From = union(enum) {
    table: []const u8,
    // future join statement
};

/// Parse a SQL SELECT query string into a Select struct.
/// Input format: "SELECT col1, col2, ... FROM table_name" or "SELECT * FROM table_name"
/// If columns is empty array in result, it means SELECT * (all columns)
pub fn parse(input: []const u8, allocator: std.mem.Allocator) !Select {
    var iter = std.mem.splitSequence(u8, input, " ");

    // Expect SELECT keyword
    const select_keyword = iter.next() orelse return Error.InvalidSyntax;
    if (!std.mem.eql(u8, try std.ascii.allocLowerString(allocator, select_keyword), "select")) {
        return Error.InvalidSyntax;
    }

    // Collect column names until we hit FROM
    var column_list: std.array_list.Managed(Column) = .init(allocator);
    defer column_list.deinit();

    // parse the table names
    while (iter.next()) |token| {
        const trimmed = std.mem.trim(u8, token, " \t\r\n,");

        if (trimmed.len == 0) continue;
        if (std.mem.eql(u8, trimmed, "*")) continue;
        if (std.mem.eql(u8, try std.ascii.allocLowerString(allocator, trimmed), "from")) break;
        try column_list.append(.{ .name = trimmed, .table = null });
    } else {
        return Error.InvalidSyntax;
    }

    // find the table name
    var table_name: []const u8 = "";
    if (iter.next()) |tbl| {
        table_name = std.mem.trim(u8, tbl, " \t\r\n");
        if (table_name.len == 0) {
            return Error.InvalidSyntax;
        }
    } else {
        return Error.InvalidSyntax;
    }

    return Select{
        .columns = try allocator.dupe(Column, column_list.items),
        .from = .{ .table = table_name },
    };
}
