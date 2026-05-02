package blizzard

import (
	"encoding/binary"
	"fmt"
	"io"
	"jph/model-export/pkg/model"
	"os"

	"github.com/go-gl/mathgl/mgl32"
)

func M2SkelFromFile(path string) (*M2Skeleton, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open skel file: %w", err)
	}
	return M2SkelFromReader(file)
}

func M2SkelFromReader(r io.Reader) (*M2Skeleton, error) {
	var skel M2Skeleton

	reader := M2ChunkReader{Reader: r}
	for header, data := range reader.Chunks {
		switch string(header.Token[:]) {
		case "SKL1":
			var header m2SKL1Header
			if _, err := binary.Decode(data, binary.LittleEndian, &header); err != nil {
				return nil, fmt.Errorf("failed to decode SKL1: %w", err)
			}
			skel.Name = string(header.Name.Load(data, 0))
		case "SKB1":
			var header m2SKB1Header
			if _, err := binary.Decode(data, binary.LittleEndian, &header); err != nil {
				return nil, fmt.Errorf("failed to decode SKB1: %w", err)
			}
			bones := header.Bones.Load(data, 0)
			skel.Bones = make([]m2LoadedBone, len(bones))
			for i, bone := range bones {
				skel.Bones[i] = bone.Load(data, 0)
			}
		case "SKS1":
			var header m2SKS1Header
			if _, err := binary.Decode(data, binary.LittleEndian, &header); err != nil {
				return nil, fmt.Errorf("failed to decode SKS1: %w", err)
			}
			skel.Sequences = header.Sequences.Load(data, 0)
			skel.SequenceIds = header.SequenceLookup.Load(data, 0)
		case "SKPD":
			var header m2SKPDHeader
			if _, err := binary.Decode(data, binary.LittleEndian, &header); err != nil {
				return nil, fmt.Errorf("failed to decode SKPD: %w", err)
			}
			skel.ParentSkelFileId = &header.ParentSkelFileId
		case "AFID":
			var scratch m2AFIDData
			skel.AnimMeta = make([]m2AFIDData, len(data)/binary.Size(scratch))
			if _, err := binary.Decode(data, binary.LittleEndian, skel.AnimMeta); err != nil {
				return nil, fmt.Errorf("failed to decode AFID: %w", err)
			}
		case "BFID":
			var scratch m2BFIDData
			skel.BoneFileIds = make([]m2BFIDData, len(data)/binary.Size(scratch))
			if _, err := binary.Decode(data, binary.LittleEndian, skel.BoneFileIds); err != nil {
				return nil, fmt.Errorf("failed to decode BFID: %w", err)
			}
		}
	}

	return &skel, nil
}

type M2Skeleton struct {
	Name             string
	Bones            []m2LoadedBone
	Sequences        []M2Sequence
	SequenceIds      []int16
	ParentSkelFileId *uint32
	AnimMeta         []m2AFIDData
	BoneFileIds      []m2BFIDData
}

func (skel M2Skeleton) FillModel(mdl *model.Model) {
	if len(skel.Sequences) > 0 {
		mdl.Animations = make([]model.Animation, len(skel.Sequences))
		for i, seq := range skel.Sequences {
			mdl.Animations[i].Name = fmt.Sprintf("%d_%d", seq.ID, seq.VariationIndex)
			mdl.Animations[i].Duration = float32(seq.Duration) / 1000.0
			mdl.Animations[i].TranslationTracks = make([]model.AnimationTrack[mgl32.Vec3], 0)
			mdl.Animations[i].RotationTracks = make([]model.AnimationTrack[mgl32.Vec4], 0)
			mdl.Animations[i].ScaleTracks = make([]model.AnimationTrack[mgl32.Vec3], 0)
		}
	}

	if len(skel.Bones) > 0 {
		mdl.Skeleton = &model.Skeleton{
			BoneNames:           make([]string, len(skel.Bones)),
			BoneParents:         make([]int, len(skel.Bones)),
			BoneInvBindMatrices: make([]mgl32.Mat4, len(skel.Bones)),
		}
		for i, bone := range skel.Bones {
			mdl.Skeleton.BoneNames[i] = fmt.Sprintf("bone_%s (%d, 0x%08x)", bone.GetName(), bone.KeyBoneId, bone.BoneNameCRC)
			mdl.Skeleton.BoneParents[i] = int(bone.ParentBone)
			mdl.Skeleton.BoneInvBindMatrices[i] = mgl32.Mat4FromRows(
				mgl32.Vec4{1.0, 0.0, 0.0, -bone.Pivot.X},
				mgl32.Vec4{0.0, 1.0, 0.0, -bone.Pivot.Y},
				mgl32.Vec4{0.0, 0.0, 1.0, -bone.Pivot.Z},
				mgl32.Vec4{0.0, 0.0, 0.0, 1.0},
			)

			for animIdx, ts := range bone.Translation.Timestamps {
				track := model.AnimationTrack[mgl32.Vec3]{
					Bone:          i,
					Interpolation: bone.Translation.InterpolationType.IntoModel(),
					Timestamps:    normalizeTimesatmps(ts, skel.Sequences[animIdx].Duration),
					Values:        make([]mgl32.Vec3, len(ts)),
				}
				for trackIdx, v := range bone.Translation.Values[animIdx] {
					track.Values[trackIdx] = v.IntoMGL32()
				}
				mdl.Animations[animIdx].TranslationTracks = append(mdl.Animations[animIdx].TranslationTracks, track)
			}

			for animIdx, ts := range bone.Rotation.Timestamps {
				track := model.AnimationTrack[mgl32.Vec4]{
					Bone:          i,
					Interpolation: bone.Rotation.InterpolationType.IntoModel(),
					Timestamps:    normalizeTimesatmps(ts, skel.Sequences[animIdx].Duration),
					Values:        make([]mgl32.Vec4, len(ts)),
				}
				for trackIdx, v := range bone.Rotation.Values[animIdx] {
					track.Values[trackIdx] = v.Decompress().IntoMGL32().Normalize()
				}
				mdl.Animations[animIdx].RotationTracks = append(mdl.Animations[animIdx].RotationTracks, track)
			}

			for animIdx, ts := range bone.Scale.Timestamps {
				track := model.AnimationTrack[mgl32.Vec3]{
					Bone:          i,
					Interpolation: bone.Scale.InterpolationType.IntoModel(),
					Timestamps:    normalizeTimesatmps(ts, skel.Sequences[animIdx].Duration),
					Values:        make([]mgl32.Vec3, len(ts)),
				}
				for trackIdx, v := range bone.Scale.Values[animIdx] {
					track.Values[trackIdx] = v.IntoMGL32()
				}
				mdl.Animations[animIdx].ScaleTracks = append(mdl.Animations[animIdx].ScaleTracks, track)
			}
		}
	}
}

func normalizeTimesatmps(ms []uint32, duration uint32) []float32 {
	ts := make([]float32, len(ms))
	for i, v := range ms {
		if duration > 0 {
			norm_ms := v % duration
			if norm_ms == 0 && v > 0 {
				v = duration
			} else {
				v = norm_ms
			}
		}
		ts[i] = float32(v) / 1000
	}
	return ts
}
