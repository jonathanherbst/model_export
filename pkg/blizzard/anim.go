package blizzard

import (
	"io"
)

func M2AnimFromReader(r io.Reader) M2Anim {
	var animData M2Anim
	reader := M2ChunkReader{r}
	for header, data := range reader.Chunks {
		switch string(header.Token[:]) {
		case "AFM2":
			animData.afm2 = data
		case "AFSA":
			animData.afsa = data
		case "AFSB":
			animData.afsb = data
		}
	}
	return animData
}

type M2Anim struct {
	afm2 []byte
	afsa []byte
	afsb []byte
}

func (anim M2Anim) BoneData() []byte {
	if len(anim.afsb) > 0 {
		return anim.afsb
	}
	return anim.afm2
}
