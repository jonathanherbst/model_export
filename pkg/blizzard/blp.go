package blizzard

import (
	"encoding/binary"
	"errors"
	"image"
	"io"
	"os"

	"github.com/mauserzjeh/dxt"
)

var (
	ErrInvalidFormat          = errors.New("blp: invalid file format")
	ErrInvalidImageFormat     = errors.New("blp: invalid image format")
	ErrUnsupportedImageFormat = errors.New("blp: unsupported image format")
)

func BLPFromFile(path string) (*BLP, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrInvalidFormat
	}
	return BLPFromReader(f)
}

func BLPFromReader(r io.ReadSeekCloser) (*BLP, error) {
	var h blpHeader
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		r.Close()
		return nil, ErrInvalidFormat
	}
	if string(h.Magic[:]) != blp2Magic || h.Version != 1 {
		r.Close()
		return nil, ErrInvalidFormat
	}
	return &BLP{header: h, reader: r}, nil
}

type BLP struct {
	header blpHeader
	reader io.ReadSeekCloser
}

func (blp BLP) Close() {
	blp.reader.Close()
}

func (blp BLP) Decode(level int) (*image.RGBA, error) {
	data, err := blp.getMipmap(level)
	if err != nil {
		return nil, ErrInvalidImageFormat
	}

	switch blp.header.ColorEncoding {
	case BLPColorDXT:
		return blp.decodeDXT(data)
	default:
		return nil, ErrUnsupportedImageFormat
	}
}

func (blp BLP) getMipmap(level int) ([]byte, error) {
	offset := blp.header.MipOffsets[level]
	size := blp.header.MipSizes[level]
	_, err := blp.reader.Seek(int64(offset), io.SeekStart)
	if err != nil {
		return nil, err
	}

	data := make([]byte, size)
	_, err = io.ReadFull(blp.reader, data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (blp BLP) decodeDXT(data []byte) (*image.RGBA, error) {
	var rgbaData []byte
	var err error
	switch blp.header.AlphaType {
	case BLPAlphaNone:
		rgbaData, err = dxt.DecodeDXT1(data, uint(blp.header.Width), uint(blp.header.Height))
	case BLPAlphaDXT3:
		rgbaData, err = dxt.DecodeDXT3(data, uint(blp.header.Width), uint(blp.header.Height))
	case BLPAlphaDXT5:
		rgbaData, err = dxt.DecodeDXT5(data, uint(blp.header.Width), uint(blp.header.Height))
	default:
		return nil, ErrInvalidImageFormat
	}
	if err != nil {
		return nil, ErrInvalidImageFormat
	}

	return &image.RGBA{
		Pix:    rgbaData,
		Stride: int(blp.header.Width * 4),
		Rect:   image.Rect(0, 0, int(blp.header.Width), int(blp.header.Height)),
	}, nil
}

const (
	blp2Magic = "BLP2"
)

type blpColorEncoding uint8

const (
	BLPColorJPEG    blpColorEncoding = 0
	BLPColorPalette blpColorEncoding = 1
	BLPColorDXT     blpColorEncoding = 2
	BLPColorBGRA    blpColorEncoding = 3
)

type blpAlphaType uint8

const (
	BLPAlphaNone blpAlphaType = 0
	BLPAlphaDXT3 blpAlphaType = 1
	BLPAlphaDXT5 blpAlphaType = 7
)

type blpHeader struct {
	Magic         [4]byte
	Version       uint32
	ColorEncoding blpColorEncoding
	AlphaBitDepth uint8
	AlphaType     blpAlphaType
	HasMipmaps    uint8
	Width         uint32
	Height        uint32
	MipOffsets    [16]uint32
	MipSizes      [16]uint32
	Palette       [256]uint32
}
