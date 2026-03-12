package blizzard

import (
	"fmt"
	casclib_sys "jph/model-export/pkg/casclib-sys"
)

type Casc struct {
	storage      casclib_sys.Storage
	ProductName  string
	BuildNumber  uint32
	ListFilePath *string
}

func OpenCasc(path string) (*Casc, error) {
	storage, err := casclib_sys.OpenStorage(path)
	if err != nil {
		return nil, fmt.Errorf("open casc failed: %w", err)
	}

	info, err := casclib_sys.GetStorageInfoProduct(storage)
	if err != nil {
		return nil, fmt.Errorf("get casc info failed: %w", err)
	}

	return &Casc{
		storage:     storage,
		ProductName: info.CodeName,
		BuildNumber: info.BuildNumber,
	}, nil
}

func (casc Casc) Close() {
	_ = casclib_sys.CloseStorage(casc.storage)
}

func (casc Casc) SearchFiles(mask string, yield func(FileData) bool) {
	iter, err := casclib_sys.FindFirstFile(casc.storage, mask, casc.ListFilePath)
	for err == nil {
		if !yield(FileData{
			Name:   iter.Name(),
			parent: casc,
		}) {
			break
		}
		err = casclib_sys.FindNextFile(iter)
	}
	casclib_sys.FindClose(iter)
}

func (casc Casc) OpenFileByName(name string, zeroEncrypted bool) (*File, error) {
	var flags uint32 = 0
	if zeroEncrypted {
		flags |= casclib_sys.OF_OVERCOME_ENCRYPTED
	}
	handle, err := casclib_sys.OpenFileByName(casc.storage, name, flags)
	if err != nil {
		return nil, fmt.Errorf("failed opening file: %w", err)
	}
	return &File{handle, name}, nil
}

func (casc Casc) OpenFileById(id uint32, zeroEncrypted bool) (*File, error) {
	var flags uint32 = 0
	if zeroEncrypted {
		flags |= casclib_sys.OF_OVERCOME_ENCRYPTED
	}
	handle, err := casclib_sys.OpenFileById(casc.storage, id, flags)
	if err != nil {
		return nil, fmt.Errorf("failed opening file: %w", err)
	}
	data, err := casclib_sys.GetFileInfo(handle)
	if err != nil {
		return nil, fmt.Errorf("failed getting file info: %w", err)
	}
	return &File{handle, data.Name()}, nil
}

type FileData struct {
	Name   string
	parent Casc
}

func (data FileData) Open(zeroEncrypted bool) (*File, error) {
	return data.parent.OpenFileByName(data.Name, zeroEncrypted)
}

type File struct {
	handle casclib_sys.File
	name   string
}

func (file File) Close() error {
	err := casclib_sys.CloseFile(file.handle)
	if err != nil {
		return fmt.Errorf("failed to close a file: %w", err)
	}
	return nil
}

func (file File) Read(buffer []byte) (uint, error) {
	len, err := casclib_sys.ReadFile(file.handle, buffer)
	if err != nil {
		return 0, fmt.Errorf("faile reading a file: %w", err)
	}
	return len, nil
}

func (file File) Seek(offset int64, whence int) (int64, error) {
	new_offset, err := casclib_sys.SetFilePointer64(file.handle, offset)
	if err != nil {
		return 0, fmt.Errorf("failed seeking a file: %w", err)
	}
	return int64(new_offset), nil
}
