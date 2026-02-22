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

fn handleCommand(casc_obj: casc.Casc, allocator: std.mem.Allocator, input: []const u8, writer: *std.Io.Writer) !void {
    var iter = std.mem.splitSequence(u8, input, " ");
    const command = iter.next() orelse return;

    if (std.mem.eql(u8, command, "exit") or std.mem.eql(u8, command, "quit")) {
        std.process.exit(0);
    } else if (std.mem.eql(u8, command, "help")) {
        try writer.print("Available commands:\n", .{});
        try writer.print("  help                - Show this help message\n", .{});
        try writer.print("  exit/quit           - Exit the utility\n", .{});
        try writer.print("  ls <path_specifier> - List files that match the path specifier\n", .{});
    } else if (std.mem.eql(u8, command, "ls")) {
        if (iter.next()) |path_specifier| {
            const path = try allocator.dupeZ(u8, path_specifier);
            defer allocator.free(path);
            try list_command(casc_obj, path, writer);
        }
    } else {
        try writer.print("Unknown command: '{s}'\n", .{command});
    }
}
