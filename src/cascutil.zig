const std = @import("std");

const casc = @import("casc.zig");
const wow = @import("wow.zig");

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    var args = std.process.args();
    const exe_path = args.next().?;
    const casc_path = args.next();
    if (casc_path == null) {
        std.debug.print("Usage: {s} <path to casc>\n", .{exe_path});
        return;
    }

    var stdin_buffer: [8 * 1024]u8 = undefined;
    var stdin_reader = std.fs.File.stdin().reader(&stdin_buffer);
    var stdin = &stdin_reader.interface;

    var stdout_buffer: [8 * 1024]u8 = undefined;
    var stdout_writer = std.fs.File.stdout().writer(&stdout_buffer);
    var stdout = &stdout_writer.interface;

    try stdout.print("Welcome to CASCUtil\n", .{});
    try stdout.print("Opening casc at: {s}\n", .{casc_path.?});
    try stdout.flush();

    const listfile_path = wow.get_best_listfile(".", allocator);
    if (listfile_path == null) {
        std.debug.print("unable to get listfile\n", .{});
        return;
    }
    defer allocator.free(listfile_path.?);

    const casc_obj = casc.Casc.open_local(casc_path.?, @ptrCast(listfile_path.?)) catch |err| {
        std.debug.print("Failed to open casclib: {}\n", .{err});
        return;
    };
    defer casc_obj.close();

    const info = try casc_obj.product_info();
    try stdout.print("Opened code_name: {s}, build: {}\n", .{ info.code_name(), info.build() });
    try stdout.print("Type 'help' for available commands or 'exit' to quit\n\n", .{});

    try stdout.print("> ", .{});
    try stdout.flush();

    while (stdin.takeDelimiterExclusive('\n')) |line| {
        // toss the delimiter
        stdin.toss(1);

        const input = std.mem.trim(u8, line, " \t\n\r");
        if (input.len > 0) {
            handleCommand(casc_obj, allocator, input, stdout) catch |err| {
                std.debug.print("Error executing command: {}\n", .{err});
            };
        }

        try stdout.print("> ", .{});
        try stdout.flush();
    } else |_| {
        try stdout.print("Goodbye!\n", .{});
    }
}

fn list_command(casc_obj: casc.Casc, path_specifier: [*:0]const u8, writer: *std.Io.Writer) !void {
    var files = try casc_obj.files(path_specifier);
    defer files.close();
    while (try files.next()) |file| {
        try writer.print("{s}\n", .{@as([*:0]const u8, @ptrCast(&file.name))});
    }
}

fn extract_command(casc_obj: casc.Casc, path_specifier: [*:0]const u8, writer: *std.Io.Writer) !void {
    var files = try casc_obj.files(path_specifier);
    defer files.close();
    while (try files.next()) |file_data| {
        const file_path: [*:0]const u8 = @ptrCast(&file_data.name);
        var file = try casc_obj.open_file(&file_data);

        const file_name = std.fs.path.basenameWindows(std.mem.span(file_path));

        const out_file = try std.fs.cwd().createFile(file_name, .{ .truncate = true });
        defer out_file.close();

        var buffer: [4096]u8 = undefined;
        var file_size: usize = 0;
        var read_len = try file.read(&buffer);
        while (read_len > 0) {
            try out_file.writeAll(buffer[0..read_len]);
            file_size += read_len;
            read_len = try file.read(&buffer);
        }

        std.debug.assert(file_size == file_data.file_size);

        try writer.print("extracted {} bytes to {s}\n", .{ file_size, file_name });
    }
}

fn tables_command(casc_obj: casc.Casc, writer: *std.Io.Writer) !void {
    var files = try casc_obj.files("*.db2");
    defer files.close();
    try writer.print("tables: ", .{});
    if (try files.next()) |file_data| {
        const file_name = std.fs.path.basenameWindows(std.mem.span(file_data.get_name()));
        const table_name = std.fs.path.stem(file_name);
        try writer.print("{s}", .{table_name});
    }
    while (try files.next()) |file_data| {
        const file_name = std.fs.path.basenameWindows(std.mem.span(file_data.get_name()));
        const table_name = std.fs.path.stem(file_name);
        try writer.print(", {s}", .{table_name});
    }
    try writer.print("\n", .{});
}

//fn select_command(casc_obj: casc.Casc, args: []const u8, writer: *std.Io.Writer) !void {}

fn handleCommand(casc_obj: casc.Casc, allocator: std.mem.Allocator, input: []const u8, writer: *std.Io.Writer) !void {
    var iter = std.mem.splitSequence(u8, input, " ");
    const command = iter.next() orelse return;

    if (std.mem.eql(u8, command, "exit") or std.mem.eql(u8, command, "quit") or std.mem.eql(u8, command, "e") or std.mem.eql(u8, command, "q")) {
        std.process.exit(0);
    } else if (std.mem.eql(u8, command, "help")) {
        try writer.print("Available commands:\n", .{});
        try writer.print("  help                - Show this help message\n", .{});
        try writer.print("  (e)xit/(q)uit       - Exit the utility\n", .{});
        try writer.print("  ls <path_specifier> - List files that match the path specifier\n", .{});
        try writer.print("  x <path>            - Extract a file to the cwd\n", .{});
        try writer.print("  tables              - List all the database table in the casc file\n", .{});
    } else if (std.mem.eql(u8, command, "ls")) {
        if (iter.next()) |path_specifier| {
            const path = try allocator.dupeZ(u8, path_specifier);
            defer allocator.free(path);
            try list_command(casc_obj, path, writer);
        }
    } else if (std.mem.eql(u8, command, "x")) {
        if (iter.next()) |path_nondelim| {
            const path = try allocator.dupeZ(u8, path_nondelim);
            defer allocator.free(path);
            try extract_command(casc_obj, path, writer);
        }
    } else if (std.mem.eql(u8, command, "tables")) {
        try tables_command(casc_obj, writer);
    } else {
        try writer.print("Unknown command: '{s}'\n", .{command});
    }
}
