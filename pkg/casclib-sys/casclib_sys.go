package casclib_sys

//go:generate sh -c "mkdir -p ../../build-casclib && cd ../../build-casclib && cmake ../casclib -DCMAKE_BUILD_TYPE=Release -DCMAKE_POLICY_VERSION_MINIMUM=3.5 -DCASC_BUILD_STATIC_LIB=ON -DCASC_BUILD_SHARED_LIB=OFF && make"

/*
#cgo CFLAGS: -I${SRCDIR}/../../casclib/src
#cgo LDFLAGS: ${SRCDIR}/../../build-casclib/libcasc.a -lstdc++ -lz -lbz2
#include "CascLib.h"

static const char* casc_file_data_id(size_t id) {
	return CASC_FILE_DATA_ID(id);
}

// expose all ERROR_* macros as typed constants for Go; cgo will
// translate each `static const` into a Go constant, even when the macro
// expands to another identifier.
enum {
    _ERROR_SUCCESS              = ERROR_SUCCESS,
    _ERROR_FILE_NOT_FOUND       = ERROR_FILE_NOT_FOUND,
    _ERROR_ACCESS_DENIED        = ERROR_ACCESS_DENIED,
    _ERROR_INVALID_HANDLE       = ERROR_INVALID_HANDLE,
    _ERROR_NOT_ENOUGH_MEMORY    = ERROR_NOT_ENOUGH_MEMORY,
    _ERROR_NOT_SUPPORTED        = ERROR_NOT_SUPPORTED,
    _ERROR_INVALID_PARAMETER    = ERROR_INVALID_PARAMETER,
    _ERROR_DISK_FULL            = ERROR_DISK_FULL,
    _ERROR_ALREADY_EXISTS       = ERROR_ALREADY_EXISTS,
    _ERROR_INSUFFICIENT_BUFFER  = ERROR_INSUFFICIENT_BUFFER,
    _ERROR_BAD_FORMAT           = ERROR_BAD_FORMAT,
    _ERROR_NO_MORE_FILES        = ERROR_NO_MORE_FILES,
    _ERROR_HANDLE_EOF           = ERROR_HANDLE_EOF,
    _ERROR_CAN_NOT_COMPLETE     = ERROR_CAN_NOT_COMPLETE,
    _ERROR_FILE_CORRUPT         = ERROR_FILE_CORRUPT,
    _ERROR_FILE_ENCRYPTED       = ERROR_FILE_ENCRYPTED,
    _ERROR_FILE_TOO_LARGE       = ERROR_FILE_TOO_LARGE,
    _ERROR_ARITHMETIC_OVERFLOW  = ERROR_ARITHMETIC_OVERFLOW,
    _ERROR_NETWORK_NOT_AVAILABLE = ERROR_NETWORK_NOT_AVAILABLE,
    _ERROR_FILE_INCOMPLETE      = ERROR_FILE_INCOMPLETE,
    _ERROR_FILE_OFFLINE         = ERROR_FILE_OFFLINE,
    _ERROR_BUFFER_OVERFLOW      = ERROR_BUFFER_OVERFLOW,
    _ERROR_CANCELLED            = ERROR_CANCELLED,
    _ERROR_INDEX_PARSING_DONE   = ERROR_INDEX_PARSING_DONE,
    _ERROR_REPARSE_ROOT         = ERROR_REPARSE_ROOT,
    _ERROR_CKEY_ALREADY_OPENED  = ERROR_CKEY_ALREADY_OPENED,
};
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"
)

// Storage is an opaque CASC handle returned by the library.
type Storage C.HANDLE

func OpenStorage(path string) (Storage, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var handle C.HANDLE
	if !C.CascOpenStorage(cpath, C.CASC_LOCALE_ALL, &handle) {
		return nil, getError()
	}
	return Storage(handle), nil
}

func OpenOnlineStorage(cache string, code string, url string, region string) (Storage, error) {
	params := cache
	if url != "" {
		params += ":" + url
	}
	params += ":" + code
	if region != "" {
		params += ":" + region
	}
	cparams := C.CString(params)
	defer C.free(unsafe.Pointer(cparams))
	var handle C.HANDLE
	if !C.CascOpenOnlineStorage(cparams, C.CASC_LOCALE_ALL, &handle) {
		return nil, getError()
	}
	return Storage(handle), nil
}

func CloseStorage(handle Storage) error {
	if !C.CascCloseStorage(C.HANDLE(handle)) {
		return getError()
	}
	return nil
}

type StorageInfoProduct struct {
	CodeName    string
	BuildNumber uint32
}

func GetStorageInfoProduct(handle Storage) (*StorageInfoProduct, error) {
	var product C.CASC_STORAGE_PRODUCT
	var product_size = unsafe.Sizeof(product)
	if !C.CascGetStorageInfo(C.HANDLE(handle), C.CascStorageProduct, unsafe.Pointer(&product), C.size_t(product_size), nil) {
		return nil, getError()
	}
	return &StorageInfoProduct{
		CodeName:    C.GoString(&product.szCodeName[0]),
		BuildNumber: uint32(product.BuildNumber),
	}, nil
}

func FindFirstFile(handle Storage, mask string, listfilePath *string) (*FileIterator, error) {
	var iter FileIterator
	cmask := C.CString(mask)
	defer C.free(unsafe.Pointer(cmask))
	var clistfilePath *C.char = nil
	if listfilePath != nil {
		clistfilePath = C.CString(*listfilePath)
		defer C.free(unsafe.Pointer(clistfilePath))
	}
	iter.handle = C.CascFindFirstFile(C.HANDLE(handle), cmask, &iter.data, clistfilePath)
	if iter.handle == C.HANDLE(C.INVALID_HANDLE_VALUE) {
		return nil, getError()
	}
	return &iter, nil
}

type File C.HANDLE

const (
	OF_OVERCOME_ENCRYPTED = uint32(C.CASC_OVERCOME_ENCRYPTED)
)

func OpenFileByName(storage Storage, name string, flags uint32) (File, error) {
	var file C.HANDLE
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	flags = (flags & 0xFFFFFFF0) | C.CASC_OPEN_BY_NAME
	if !C.CascOpenFile(C.HANDLE(storage), unsafe.Pointer(cname), 0, C.DWORD(flags), &file) {
		return nil, getError()
	}
	return File(file), nil
}

func OpenFileById(storage Storage, id uint32, flags uint32) (File, error) {
	var file C.HANDLE
	file_name := C.casc_file_data_id(C.size_t(id))
	flags = (flags & 0xFFFFFFF0) | C.CASC_OPEN_BY_FILEID
	if !C.CascOpenFile(C.HANDLE(storage), unsafe.Pointer(file_name), 0, C.DWORD(flags), &file) {
		return nil, getError()
	}
	return File(file), nil
}

func CloseFile(file File) error {
	if !C.CascCloseFile(C.HANDLE(file)) {
		return getError()
	}
	return nil
}

func ReadFile(file File, buffer []byte) (int, error) {
	var bytes_read C.DWORD
	if !C.CascReadFile(C.HANDLE(file), unsafe.Pointer(&buffer[0]), C.DWORD(len(buffer)), &bytes_read) {
		return 0, getError()
	}
	return int(bytes_read), nil
}

func SetFilePointer64(file File, off int64) (uint64, error) {
	var new_offset C.ULONGLONG
	if !C.CascSetFilePointer64(C.HANDLE(file), C.LONGLONG(off), &new_offset, C.FILE_BEGIN) {
		return 0, getError()
	}
	return uint64(new_offset), nil
}

func GetFileInfo(file File) (*FileInfo, error) {
	var data C.CASC_FILE_FULL_INFO
	if !C.CascGetFileInfo(C.HANDLE(file), C.CascFileFullInfo, unsafe.Pointer(&data), C.size_t(unsafe.Sizeof(data)), nil) {
		return nil, getError()
	}
	return &FileInfo{data}, nil
}

type FileInfo struct {
	data C.CASC_FILE_FULL_INFO
}

func (info FileInfo) Name() string {
	return C.GoString(&info.data.DataFileName[0])
}

type FileIterator struct {
	handle C.HANDLE
	data   C.CASC_FIND_DATA
}

func (iter FileIterator) Name() string {
	return C.GoString(&iter.data.szFileName[0])
}

func FindNextFile(iter *FileIterator) error {
	var p runtime.Pinner
	defer p.Unpin()
	p.Pin(iter)
	if !C.CascFindNextFile(iter.handle, &iter.data) {
		return getError()
	}
	return nil
}

func FindClose(iter *FileIterator) error {
	if !C.CascFindClose(iter.handle) {
		return getError()
	}
	return nil
}

// Get the error when a function returns nil, or false
func getError() error {
	err := int(C.GetCascError())
	return fmt.Errorf("%s - %d", errorCodeNames[err], err)
}

var (
	ErrorSuccess            = int(C._ERROR_SUCCESS)
	ErrorAccessDenied       = int(C._ERROR_ACCESS_DENIED)
	ErrorFileNotFound       = int(C._ERROR_FILE_NOT_FOUND)
	ErrorInvalidHandle      = int(C._ERROR_INVALID_HANDLE)
	ErrorNotEnoughMemory    = int(C._ERROR_NOT_ENOUGH_MEMORY)
	ErrorAlreadyExists      = int(C._ERROR_ALREADY_EXISTS)
	ErrorInvalidParameter   = int(C._ERROR_INVALID_PARAMETER)
	ErrorDiskFull           = int(C._ERROR_DISK_FULL)
	ErrorNotSupported       = int(C._ERROR_NOT_SUPPORTED)
	ErrorInsufficientBuffer = int(C._ERROR_INSUFFICIENT_BUFFER)
	ErrorBadFormat          = int(C._ERROR_BAD_FORMAT)
	ErrorNoMoreFiles        = int(C._ERROR_NO_MORE_FILES)
	ErrorHandleEOF          = int(C._ERROR_HANDLE_EOF)
	ErrorCanNotComplete     = int(C._ERROR_CAN_NOT_COMPLETE)
	ErrorFileCorrupt        = int(C._ERROR_FILE_CORRUPT)
	ErrorFileEncrypted      = int(C._ERROR_FILE_ENCRYPTED)
	ErrorFileTooLarge       = int(C._ERROR_FILE_TOO_LARGE)
	ErrorFileOffline        = int(C._ERROR_FILE_OFFLINE)
	ErrorBufferOverflow     = int(C._ERROR_BUFFER_OVERFLOW)
	ErrorCancelled          = int(C._ERROR_CANCELLED)
	ErrorIndexParsingDone   = int(C._ERROR_INDEX_PARSING_DONE)
	ErrorReparseRoot        = int(C._ERROR_REPARSE_ROOT)
	ErrorCkeyAlreadyOpened  = int(C._ERROR_CKEY_ALREADY_OPENED)
)

// errorCodeNames maps each unique ErrorCode to a canonical name.  When
// multiple Go constants share the same numeric value (e.g. FileNotFound
// and PathNotFound both map to ENOENT) the later entry overrides the
// previous one; we intentionally list them in the same order as the
// definitions above so that the more generic name wins.
var errorCodeNames = map[int]string{
	ErrorSuccess:            "not an error",
	ErrorFileNotFound:       "file not found",
	ErrorAccessDenied:       "access denied",
	ErrorInvalidHandle:      "invalid handle",
	ErrorNotEnoughMemory:    "not enough memory",
	ErrorNotSupported:       "not supported",
	ErrorInvalidParameter:   "invalid parameter",
	ErrorDiskFull:           "disk full",
	ErrorAlreadyExists:      "already exists",
	ErrorInsufficientBuffer: "insufficient buffer",
	ErrorBadFormat:          "bad format",
	ErrorNoMoreFiles:        "no more files",
	ErrorHandleEOF:          "end of file",
	ErrorCanNotComplete:     "cannot complete",
	ErrorFileCorrupt:        "corrupt file",
	ErrorFileEncrypted:      "encrypted file",
	ErrorFileTooLarge:       "file too large",
	ErrorFileOffline:        "file offline",
	ErrorCancelled:          "cancelled",
	ErrorIndexParsingDone:   "index parsing done",
	ErrorReparseRoot:        "reparse root",
	ErrorCkeyAlreadyOpened:  "ckey already opened",
}
