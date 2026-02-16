const builtin = @import("builtin");
const std = @import("std");

const casc = @import("casc.zig");
const wow = @import("wow.zig");

const Directories = struct {
    allocator: std.mem.Allocator,
    app_dir: []u8,
    cache_dir: []u8,

    pub fn get(allocator: std.mem.Allocator) !Directories {
        const app_dir = try std.fs.getAppDataDir(allocator, "model-export");
        errdefer allocator.free(app_dir);
        var cache_dir: []u8 = undefined;
        if (builtin.target.os.tag == .windows) {
            cache_dir = try std.fs.path.join(allocator, &[_][]const u8{ app_dir, "cache" });
        } else if (builtin.target.os.tag == .macos) {
            if (std.posix.getenv("HOME")) |home| {
                cache_dir = try std.fs.path.join(allocator, &[_][]const u8{ home, "Library/Caches/model-export" });
            } else {
                cache_dir = try std.fs.path.std.fs.cwd().realpathAlloc(allocator, "");
            }
        } else {
            if (std.posix.getenv("XDG_CACHE_HOME")) |xdg_cache_dir| {
                cache_dir = try std.fs.path.join(allocator, &[_][]const u8{ xdg_cache_dir, "model-export" });
            } else if (std.posix.getenv("HOME")) |home| {
                cache_dir = try std.fs.path.join(allocator, &[_][]const u8{ home, ".cache/model-export" });
            } else {
                cache_dir = try std.fs.cwd().realpathAlloc(allocator, "");
            }
        }
        return Directories{
            .allocator = allocator,
            .app_dir = app_dir,
            .cache_dir = cache_dir,
        };
    }

    pub fn deinit(self: Directories) void {
        self.allocator.free(self.app_dir);
        self.allocator.free(self.cache_dir);
    }
};

pub fn main() !void {
    var arena = std.heap.ArenaAllocator.init(std.heap.page_allocator);
    defer arena.deinit();
    const allocator = arena.allocator();

    const dirs = try Directories.get(allocator);
    defer dirs.deinit();
    std.debug.print("app dir: {s}\n", .{dirs.app_dir});
    std.debug.print("cache dir: {s}\n", .{dirs.cache_dir});

    var args = try std.process.argsWithAllocator(allocator);
    defer args.deinit();

    const wow_cache_dir = try std.fs.path.join(allocator, &[_][]const u8{ dirs.cache_dir, "wow" });
    defer allocator.free(wow_cache_dir);

    if (wow.get_best_listfile(wow_cache_dir, allocator)) |path| {
        defer allocator.free(path);
        std.debug.print("best listfile: {s}\n", .{path});
    }

    if (wow.get_best_dbd_package(wow_cache_dir, allocator)) |path| {
        defer allocator.free(path);
        std.debug.print("best dbd package: {s}\n", .{path});
    }

    // const releases = try github.Releases.fromUrl("https://api.github.com/repos/wowdev/wow-listfile/releases", allocator);
    // defer releases.deinit();
    // if (releases.latest()) |release| {
    //     std.debug.print("latest release created at {s}\n", .{release.created_at});
    // } else {
    //     std.debug.print("no releases found", .{});
    // }

    // if (args.skip()) {
    //     if (args.next()) |wow_dir| {
    //         const wow = try casc.Casc.open_local(wow_dir);
    //         defer wow.close();

    //         var files = try wow.files("*.db2", "verified-listfile.csv");
    //         defer files.close();

    //         if (try files.next()) |file_data| {
    //             const wow_path = @as([*:0]const u8, @ptrCast(&file_data.name));
    //             const file_name = std.fs.path.basenameWindows(std.mem.span(wow_path));
    //             const file = try wow.open_file(&file_data);
    //             defer file.close();

    //             std.debug.print("file: {s}, {} bytes\n", .{ file_name, try file.size() });
    //             const out_file = try std.fs.cwd().createFile(file_name, .{ .truncate = true });
    //             defer out_file.close();

    //             var buffer: [4096]u8 = undefined;
    //             var read_len = try file.read(&buffer);
    //             while (read_len > 0) {
    //                 try out_file.writeAll(buffer[0..read_len]);
    //                 read_len = try file.read(&buffer);
    //             }
    //         }

    //         // while (try files.next()) |file_data| {
    //         //     std.debug.print("{s}\n", .{@as([*:0]const u8, @ptrCast(&file_data.name))});
    //         // }
    //     }
    // }
}
