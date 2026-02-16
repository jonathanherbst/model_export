const std = @import("std");
const github = @import("github.zig");

pub fn get_best_listfile(cache_dir: []const u8, allocator: std.mem.Allocator) ?[]const u8 {
    const path = std.fs.path.join(allocator, &[_][]const u8{ cache_dir, "verified-listfile.csv" }) catch |err| {
        std.debug.panic("couldn't allocate join {}", .{err});
    };

    if (github.fetch_latest_release(path, "https://api.github.com/repos/wowdev/wow-listfile/releases", allocator)) {
        return path;
    }
    return null;
}

pub fn get_best_dbd_package(cache_dir: []const u8, allocator: std.mem.Allocator) ?[]const u8 {
    const path = std.fs.path.join(allocator, &[_][]const u8{ cache_dir, "dbd.zip" }) catch |err| {
        std.debug.panic("couldn't allocate join {}", .{err});
    };

    if (github.fetch_latest_release(path, "https://api.github.com/repos/wowdev/WoWDBDefs/releases", allocator)) {
        return path;
    }
    return null;
}
