const std = @import("std");

const casc = @import("casc.zig");
const dbd = @import("dbd.zig");
const github = @import("github.zig");
const wdc5 = @import("wdc5.zig");

const log = std.log.scoped(.wow);

const Error = error{
    UnableToGetListfile,
    UnableToGetDBDData,
    UnknownTable,
};

pub fn get_best_listfile(cache_dir: []const u8, allocator: std.mem.Allocator) ?[]const u8 {
    const path = std.fs.path.join(allocator, &[_][]const u8{ cache_dir, "verified-listfile-withcapitals.csv" }) catch |err| {
        std.debug.panic("couldn't allocate join {}", .{err});
    };

    if (github.fetch_latest_release(path, "https://api.github.com/repos/wowdev/wow-listfile/releases", allocator)) {
        return path;
    }
    allocator.free(path);
    return null;
}

pub fn get_best_dbd_package(cache_dir: []const u8, allocator: std.mem.Allocator) ?[]const u8 {
    const path = std.fs.path.join(allocator, &[_][]const u8{ cache_dir, "dbd.zip" }) catch |err| {
        std.debug.panic("couldn't allocate join {}", .{err});
    };
    defer allocator.free(path);

    if (github.fetch_latest_release(path, "https://api.github.com/repos/wowdev/WoWDBDefs/releases", allocator)) {
        const extract_path = std.fs.path.join(allocator, &[_][]const u8{ cache_dir, "dbd" }) catch |err| {
            std.debug.panic("couldn't allocate join {}", .{err});
        };

        // The zip library doesn't support inspecting files before extracting and errors if files already exist.
        // so the best solution I could come up with is to delete the folder and rextract always.
        std.fs.cwd().deleteTree(extract_path) catch {};
        std.fs.cwd().makePath(extract_path) catch |err| {
            log.err("unable to create the dbd cache path {}", .{err});
            allocator.free(extract_path);
            return null;
        };
        const extract_dir = std.fs.cwd().openDir(extract_path, .{}) catch |err| {
            std.debug.panic("can't open the dbd cache dir {}", .{err});
        };

        var file = std.fs.cwd().openFile(path, .{}) catch |err| {
            std.debug.panic("can't open the file we just downloaded {}", .{err});
        };
        var buffer: [4096]u8 = undefined;
        var reader = file.reader(&buffer);
        std.zip.extract(extract_dir, &reader, .{}) catch |err| {
            log.err("failed to extract {s}, {}", .{ path, err });
            allocator.free(extract_path);
            return null;
        };
        return extract_path;
    }
    return null;
}

const CascDatabaseParams = struct {
    allocator: std.mem.Allocator = std.heap.page_allocator,
    cache_dir: []const u8 = ".",
};

pub const CascDatabase = struct {
    casc: *casc.Casc,
    arena: std.heap.ArenaAllocator,
    dbd_dir: []const u8,
    tables: TableMap,

    pub fn is_wow_casc(casc_obj: casc.Casc) bool {
        const prod_info = casc_obj.product_info() catch {
            return false;
        };
        return std.mem.startsWith(u8, std.mem.span(prod_info.code_name()), "wow");
    }

    pub fn open_casc(casc_obj: *casc.Casc, params: CascDatabaseParams) !@This() {
        std.debug.assert(is_wow_casc(casc_obj.*));

        var arena: std.heap.ArenaAllocator = .init(params.allocator);
        errdefer arena.deinit();
        var allocator = arena.allocator();

        const list_file = get_best_listfile(params.cache_dir, allocator) orelse {
            return Error.UnableToGetListfile;
        };
        const dbd_dir = get_best_dbd_package(params.cache_dir, allocator) orelse {
            return Error.UnableToGetDBDData;
        };

        const listfile_path = allocator.dupeZ(u8, list_file) catch {
            std.debug.panic("failed to allocate memory", .{});
        };
        allocator.free(list_file);
        casc_obj.set_listfile(listfile_path);

        // load all the database files with names.
        var tables: TableMap = .init(allocator);
        var file_iter = try casc_obj.files("*.db2");
        while (try file_iter.next()) |file| {
            const path = allocator.dupeZ(u8, std.mem.span(file.get_name())) catch {
                std.debug.panic("failed to allocate memory", .{});
            };
            const table_name = std.fs.path.stem(std.fs.path.basenameWindows(path));
            // maybe load up the dbd defs with this too?
            tables.put(table_name, path) catch |err| {
                std.debug.panic("failed to put a table into the table map: {}", .{err});
            };
        }

        // Todo: load the tact encryption keys from the tactkey db2 files in the casc.

        return .{ .casc = casc_obj, .arena = arena, .dbd_dir = dbd_dir, .tables = tables };
    }

    pub fn close(self: @This()) void {
        self.casc.close();
        self.arena.deinit();
    }

    pub fn table_names(self: @This()) TableMap.KeyIterator {
        return self.tables.keyIterator();
    }

    pub fn open_table(self: *@This(), name: []const u8) !Table {
        var arena: std.heap.ArenaAllocator = .init(self.arena.allocator());
        errdefer arena.deinit();
        var allocator = arena.allocator();

        if (self.tables.get(name)) |db2_name| {
            var casc_file = try allocator.create(casc.File);
            casc_file.* = try self.casc.open_file_by_name(db2_name, .{});
            errdefer casc_file.close();

            const reader = wdc5.FileReader.from_casc_file(casc_file);
            var wdc5_file = try wdc5.File.open(reader, allocator);
            errdefer wdc5_file.close();

            const layout_hash_str = try std.fmt.allocPrint(allocator, "{X:08}", .{wdc5_file.get_layout_hash()});
            defer allocator.free(layout_hash_str);

            const dbd_file_name = try std.fmt.allocPrint(allocator, "{s}.dbd", .{name});
            defer allocator.free(dbd_file_name);
            const dbd_path = try std.fs.path.join(allocator, &.{ self.dbd_dir, dbd_file_name });
            defer allocator.free(dbd_path);
            const dbd_def = try dbd.DBD.from_reader(dbd_path, .{ .layout = layout_hash_str }, allocator);

            return .{
                .arena = arena,
                .db2 = wdc5_file,
                .def = dbd_def,
                .casc_file = casc_file,
            };
        }
        return Error.UnknownTable;
    }

    pub fn open_file(self: @This(), file_data_id: u32) !casc.File {
        return try self.casc.open_file_by_id(file_data_id, .{});
    }
};

const Table = struct {
    arena: std.heap.ArenaAllocator,
    db2: wdc5.File,
    def: dbd.DBD,
    casc_file: *casc.File,

    pub fn close(self: *@This()) void {
        self.db2.close();
        self.casc_file.close();
        self.arena.deinit();
    }
};

const TableMap = std.StringHashMap([:0]const u8);
