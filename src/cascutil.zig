const std = @import("std");

pub fn main() !void {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    var stdin_buffer: [8 * 1024]u8 = undefined;
    var stdin_reader = std.fs.File.stdin().reader(&stdin_buffer);
    var stdin = &stdin_reader.interface;

    var stdout_buffer: [8 * 1024]u8 = undefined;
    var stdout_writer = std.fs.File.stdout().writer(&stdout_buffer);
    var stdout = &stdout_writer.interface;

    //var reader = stdin.interface;

    try stdout.print("Welcome to CASCUtil\n", .{});
    try stdout.print("Type 'help' for available commands or 'exit' to quit\n\n", .{});

    try stdout.print(">> ", .{});
    try stdout.flush();

    while (stdin.takeDelimiterExclusive('\n')) |line| {
        // toss the delimiter
        stdin.toss(1);

        const input = std.mem.trim(u8, line, " \t\n\r");
        if (input.len > 0) {
            handleCommand(allocator, input, stdout) catch |err| {
                std.debug.print("Error executing command: {}\n", .{err});
            };
        }

        try stdout.print(">> ", .{});
        try stdout.flush();
    } else |_| {
        try stdout.print("Goodbye!\n", .{});
    }
}

fn handleCommand(allocator: std.mem.Allocator, input: []const u8, writer: *std.io.Writer) !void {
    var iter = std.mem.splitSequence(u8, input, " ");
    const command = iter.next() orelse return;

    if (std.mem.eql(u8, command, "exit") or std.mem.eql(u8, command, "quit")) {
        std.process.exit(0);
    } else if (std.mem.eql(u8, command, "help")) {
        try writer.print("Available commands:\n", .{});
        try writer.print("  help          - Show this help message\n", .{});
        try writer.print("  exit/quit     - Exit the utility\n", .{});
        try writer.print("  echo [text]   - Echo text back\n", .{});
    } else if (std.mem.eql(u8, command, "echo")) {
        var args = std.array_list.Managed([]const u8).init(allocator);
        defer args.deinit();

        while (iter.next()) |arg| {
            try args.append(arg);
        }

        for (args.items) |arg| {
            try writer.print("{s} ", .{arg});
        }
        try writer.print("\n", .{});
    } else {
        try writer.print("Unknown command: '{s}'\n", .{command});
    }
}
