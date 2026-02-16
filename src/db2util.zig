const std = @import("std");

const clap = @import("clap");

const dbd = @import("dbd.zig");
const wdc5 = @import("wdc5.zig");

const Stdout = struct {
    buffer: []u8,
    writer: std.fs.File.Writer,
    allocator: std.mem.Allocator,

    pub fn open(allocator: std.mem.Allocator) !@This() {
        const buffer = try allocator.alloc(u8, 1024);
        var writer = std.fs.File.stdout().writer(buffer);
        try writer.file.lock(.exclusive);
        return .{
            .buffer = buffer,
            .writer = writer,
            .allocator = allocator,
        };
    }

    pub fn close(self: @This()) void {
        self.writer.file.close();
        self.allocator.free(self.buffer);
    }

    pub fn print(self: *@This(), comptime fmt: []const u8, args: anytype) void {
        self.writer.interface.print(fmt, args) catch {};
        if (std.mem.endsWith(u8, fmt, "\n")) {
            self.writer.interface.flush() catch {};
        }
    }
};

fn print_db2_info(wdc5_file: wdc5.File, maybe_dbd_def: ?dbd.DBD, writer: *Stdout) void {
    writer.print("layout: 0x{x}, schema: {s}, flags: 0x{x}\n", .{ wdc5_file.get_layout_hash(), wdc5_file.get_schema_str(), wdc5_file.header.flags });
    writer.print("{} section(s), {} records of {} bytes with {} fields\n", .{ wdc5_file.header.section_count, wdc5_file.header.record_count, wdc5_file.header.record_size, wdc5_file.header.field_count });
    writer.print("{} bytes of pallet data, {} bytes of common data\n", .{ wdc5_file.header.pallet_data_size, wdc5_file.header.common_data_size });

    writer.print("field info: id idx({})\n", .{wdc5_file.header.id_index});
    var idx: usize = 0;
    for (wdc5_file.field_storage_infos()) |field| {
        if (maybe_dbd_def) |dbd_def| {
            // skip all the noninline columns
            while (dbd_def.columns.items[idx].annotations.noninline) {
                idx += 1;
            }
            writer.print("\t{s}({}) - ", .{ dbd_def.columns.items[idx].name, dbd_def.columns.items[idx].field_type });
            idx += 1;
        } else {
            writer.print("\t", .{});
        }
        writer.print("offset: {} bits, size: {} bits, type: {}\n", .{ field.field_offset_bits, field.field_size_bits, field.storage_type });
    }

    writer.print("sections:\n", .{});
    for (wdc5_file.section_headers()) |sec| {
        writer.print("\thash: 0x{x}, records: {}, noninline ids: {}, rel data: {} bytes, copy entries: {}\n", .{ sec.tact_key_hash, sec.record_count, sec.id_list_size, sec.relationship_data_size, sec.copy_table_count });
    }
}

fn print_records(wdc5_file: *wdc5.File, stdout: *Stdout) !void {
    var records = try wdc5_file.records();
    while (records.next()) |record| {
        stdout.print("{}:", .{record.get_id()});
        for (0..record.num_fields()) |idx| {
            const field = record.get_field(idx);
            switch (field) {
                .bytes => |v| stdout.print(" {X}", .{v}),
                .indexed => |v| {
                    stdout.print(" [{}", .{v[0]});
                    for (v[1..]) |num| {
                        stdout.print(", {}", .{num});
                    }
                    stdout.print("]", .{});
                },
                .signed => |v| stdout.print(" {}", .{v}),
                .unsigned => |v| stdout.print(" {}", .{v}),
            }
        }
        stdout.print("\n", .{});
    }
}

pub fn main() !void {
    const allocator = std.heap.page_allocator;
    var stdout = try Stdout.open(allocator);
    defer stdout.close();

    const params = comptime clap.parseParamsComptime(
        \\-h, --help    Display this help and exit.
        \\--records     Print all the records in the db2 file.
        \\--dbd <str>   Path to a dbd file that defines the schema for the db2 file.
        \\<str>         Path to a db2 file.
    );

    var diag = clap.Diagnostic{};
    var res = clap.parse(clap.Help, &params, clap.parsers.default, .{
        .diagnostic = &diag,
        .allocator = allocator,
    }) catch |err| {
        try diag.reportToFile(.stderr(), err);
        return err;
    };
    defer res.deinit();

    if (res.args.help != 0 or res.positionals[0] == null) {
        return clap.helpToFile(.stderr(), clap.Help, &params, .{});
    }

    const path = res.positionals[0].?;
    var file = try std.fs.cwd().openFile(path, .{});
    defer file.close();

    const reader = wdc5.FileReader.from_file(&file);
    var wdc5_file = try wdc5.File.open(reader, allocator);

    var dbd_def: ?dbd.DBD = null;
    if (res.args.dbd) |dbd_path| {
        var layout_str: [8]u8 = undefined;
        var writer = std.Io.Writer.fixed(&layout_str);
        try writer.print("{X}", .{wdc5_file.get_layout_hash()});
        const selector: dbd.SchemaSelector = .{ .layout = &layout_str };
        dbd_def = try dbd.DBD.from_reader(dbd_path, selector, allocator);
    }

    if (res.args.records != 0) {
        try print_records(&wdc5_file, &stdout);
    } else {
        print_db2_info(wdc5_file, dbd_def, &stdout);
    }
}
