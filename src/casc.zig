const std = @import("std");

const casclib = @cImport({
    @cInclude("CascLib.h");
});

pub const Error = error{
    FileNotFound,
    AccessDenied,
    InvalidHandle,
    NotEnoughMemory,
    NotSupported,
    InvalidParameter,
    DiskFull,
    AlreadyExists,
    InsufficientBuffer,
    BadFormat,
    NoMoreFiles,
    HandleEOF,
    CanNotComplete,
    FileCorrupt,
    FileEncrypted,
    FileTooLarge,
    ArithmeticOverflow,
    NetworkNotAvailable,
    Unknown,
};

pub const Casc = struct {
    handle: casclib.HANDLE,
    listfile_path: [*:0]const u8,

    pub fn open_local(path: [*:0]const u8, listfile_path: [*:0]const u8) Error!@This() {
        var handle: casclib.HANDLE = casclib.INVALID_HANDLE_VALUE;
        if (casclib.CascOpenStorage(path, 0, &handle) and handle != casclib.INVALID_HANDLE_VALUE) {
            return .{ .handle = handle, .listfile_path = listfile_path };
        } else {
            const code = try get_error();
            std.log.warn("got an unknown error: {}", .{code});
            return Error.Unknown;
        }
    }

    pub fn close(self: @This()) void {
        _ = casclib.CascCloseStorage(self.handle);
    }

    pub fn product_info(self: Casc) !ProductInfo {
        var data: ProductInfo = undefined;
        var output_size: usize = 0;
        if (casclib.CascGetStorageInfo(self.handle, casclib.CascStorageProduct, &data.inner, @sizeOf(casclib.CASC_STORAGE_PRODUCT), &output_size)) {
            std.debug.assert(output_size == @sizeOf(casclib.CASC_STORAGE_PRODUCT));
            return data;
        } else {
            return get_error_or_unknown();
        }
    }

    pub fn files(self: Casc, mask: [*:0]const u8) !FileSequence {
        var file_data: FileData = undefined;
        const handle: casclib.HANDLE = casclib.CascFindFirstFile(self.handle, mask, @ptrCast(&file_data), self.listfile_path);
        if (handle != casclib.INVALID_HANDLE_VALUE) {
            return FileSequence{ .handle = handle, .data = file_data };
        } else {
            _ = try get_error();
            return Error.FileNotFound;
        }
    }

    pub fn open_file(self: Casc, data: *const FileData) !File {
        var file_handle: casclib.HANDLE = null;
        if (casclib.CascOpenFile(self.handle, &data.ckey, casclib.CASC_LOCALE_NONE, casclib.CASC_OPEN_BY_CKEY, &file_handle) and
            file_handle != casclib.INVALID_HANDLE_VALUE)
        {
            return File{ .handle = file_handle };
        } else {
            _ = try get_error();
            return Error.FileNotFound;
        }
    }
};

pub const File = struct {
    handle: casclib.HANDLE,

    pub fn close(self: File) void {
        _ = casclib.CascCloseFile(self.handle);
    }

    pub fn size(self: File) !u64 {
        var file_size: c_ulonglong = 0;
        if (casclib.CascGetFileSize64(self.handle, &file_size)) {
            return file_size;
        } else {
            return get_error_or_unknown();
        }
    }

    pub fn seek(self: File, dis: i64) !u64 {
        var new_pos: c_ulonglong = 0;
        if (casclib.CascSetFilePointer64(self.handle, dis, &new_pos, casclib.FILE_BEGIN)) {
            return new_pos;
        } else {
            return get_error_or_unknown();
        }
    }

    pub fn read(self: File, buffer: []u8) !usize {
        var bytes_read: c_uint = 0;
        if (casclib.CascReadFile(self.handle, buffer.ptr, @intCast(buffer.len), &bytes_read)) {
            return bytes_read;
        } else {
            return get_error_or_unknown();
        }
    }
};

pub const FileSequence = struct {
    handle: casclib.HANDLE,
    data: ?FileData,

    pub fn next(self: *FileSequence) !?FileData {
        if (self.data) |file_data| {
            const return_data = file_data;
            if (!casclib.CascFindNextFile(self.handle, @ptrCast(&self.data))) {
                self.data = null;
                if (get_error()) |_| {} else |err| {
                    if (err != Error.NoMoreFiles) {
                        return err;
                    }
                }
            }
            return return_data;
        }
        return null;
    }

    pub fn close(self: FileSequence) void {
        _ = casclib.CascFindClose(self.handle);
    }
};

const ProductInfo = struct {
    inner: casclib.CASC_STORAGE_PRODUCT,

    pub fn code_name(self: *const @This()) [*:0]const u8 {
        std.debug.assert(std.mem.indexOfScalar(u8, &self.inner.szCodeName, 0) != null);
        return @as([*:0]const u8, @ptrCast(&self.inner.szCodeName));
    }

    pub fn build(self: @This()) u32 {
        return self.inner.BuildNumber;
    }
};

inline fn get_error_or_unknown() Error {
    const code = try get_error();
    std.log.warn("got an unknown error: {}", .{code});
    return Error.Unknown;
}

fn get_error() Error!c_uint {
    const err = casclib.GetCascError();
    return switch (err) {
        casclib.ENOENT => Error.FileNotFound,
        casclib.EPERM => Error.AccessDenied,
        casclib.EBADF => Error.InvalidHandle,
        casclib.ENOMEM => Error.NotEnoughMemory,
        casclib.ENOTSUP => Error.NotSupported,
        casclib.EINVAL => Error.InvalidParameter,
        casclib.ENOSPC => Error.DiskFull,
        casclib.EEXIST => Error.AlreadyExists,
        casclib.ENOBUFS => Error.InsufficientBuffer,
        1000 => Error.BadFormat,
        1001 => Error.NoMoreFiles,
        1002 => Error.HandleEOF,
        1003 => Error.CanNotComplete,
        1004 => Error.FileCorrupt,
        1005 => Error.FileEncrypted, // Returned by encrypted stream when can't find file key
        1006 => Error.FileTooLarge,
        1007 => Error.ArithmeticOverflow, // The string value is too large to fit in the given type
        1008 => Error.NetworkNotAvailable, // Cannot connect to the internet
        else => err,
    };
}

const FileData = extern struct {
    name: [casclib.MAX_PATH]u8,
    ckey: [casclib.MD5_HASH_SIZE]u8,
    ekey: [casclib.MD5_HASH_SIZE]u8,
    tag_bit_mask: c_ulonglong,
    file_size: c_ulonglong,
    plain_name: [*:0]u8,
    file_data_id: c_uint,
    locale_flags: c_uint,
    content_flags: c_uint,
    span_count: c_uint,
    file_available: c_uint,
    name_type: NameType,
};

const NameType = enum(c_int) {
    CascNameFull, // Fully qualified file name
    CascNameDataId, // Name created from file data id (CASC_FILEID_FORMAT)
    CascNameCKey, // Name created as string representation of CKey
    CascNameEKey, // Name created as string representation of EKey
};
