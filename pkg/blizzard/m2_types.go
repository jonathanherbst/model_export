package blizzard

import (
	"encoding/binary"
	"jph/model-export/pkg/model"
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

type m2Array[T any] struct {
	Size, Offset uint32
}

func (arr m2Array[T]) Load(buf []byte, adj int) []T {
	if arr.Size == 0 {
		return nil
	}

	data := buf[int(arr.Offset)+adj:]
	output := make([]T, arr.Size)
	size, err := binary.Decode(data, binary.LittleEndian, output)
	if err != nil {
		return nil
	}

	var t T
	if binary.Size(&t)*int(arr.Size) != size {
		panic("didn't get the entire array")
	}

	return output
}

type m2LoadedBone struct {
	KeyBoneId   int32
	Flags       m2CompBoneFlag
	ParentBone  int16  // Parent bone ID or -1 if there is none.
	SubmeshId   uint16 // Mesh part ID OR uDistToParent?
	BoneNameCRC uint32 // or CompressData, these are for debugging only. their bone names match those in key bone
	Translation m2LoadedTrack[C3Vector]
	Rotation    m2LoadedTrack[M2CompQuat]
	Scale       m2LoadedTrack[C3Vector]
	Pivot       C3Vector
}

func (bone m2LoadedBone) GetName() string {
	return LookupBoneName(bone.KeyBoneId, bone.BoneNameCRC)
}

type m2CompBone struct {
	KeyBoneId   int32 // Back-reference to the key bone lookup table. -1 if this is no key bone.
	Flags       m2CompBoneFlag
	ParentBone  int16  // Parent bone ID or -1 if there is none.
	SubmeshId   uint16 // Mesh part ID OR uDistToParent?
	BoneNameCRC uint32 // or CompressData, these are for debugging only. their bone names match those in key bone
	Translation m2Track[C3Vector]
	Rotation    m2Track[M2CompQuat]
	Scale       m2Track[C3Vector]
	Pivot       C3Vector
}

func (bone m2CompBone) Load(buf []byte, adj int) m2LoadedBone {
	translation := bone.Translation.Load(buf, adj)
	rotation := bone.Rotation.Load(buf, adj)
	scale := bone.Scale.Load(buf, adj)

	return m2LoadedBone{
		bone.KeyBoneId,
		bone.Flags,
		bone.ParentBone,
		bone.SubmeshId,
		bone.BoneNameCRC,
		translation,
		rotation,
		scale,
		bone.Pivot,
	}
}

type m2CompBoneFlag uint32

const (
	ignoreParentTranslate        m2CompBoneFlag = 0x1
	ignoreParentScale            m2CompBoneFlag = 0x2
	ignoreParentRotation         m2CompBoneFlag = 0x4
	spherical_billboard          m2CompBoneFlag = 0x8
	cylindrical_billboard_lock_x m2CompBoneFlag = 0x10
	cylindrical_billboard_lock_y m2CompBoneFlag = 0x20
	cylindrical_billboard_lock_z m2CompBoneFlag = 0x40
	transformed                  m2CompBoneFlag = 0x200
	kinematic_bone               m2CompBoneFlag = 0x400  // MoP+: allow physics to influence this bone
	helmet_anim_scaled           m2CompBoneFlag = 0x1000 // set blend_modificator to helmetAnimScalingRec.m_amount for this bone
	something_sequence_id        m2CompBoneFlag = 0x2000 // <=bfa+, parent_bone+submesh_id are a sequence id instead?!
)

type Interpolation uint16

const (
	InterpolationStep         Interpolation = 0
	InterpolationLinear       Interpolation = 1
	InterpolationCubicBezier  Interpolation = 2
	InterpolationCubicHermite Interpolation = 3
)

func (interp Interpolation) IntoModel() model.InterpolationType {
	switch interp {
	case InterpolationStep:
		return model.InterpolationStep
	case InterpolationLinear:
		return model.InterpolationLinear
	case InterpolationCubicBezier:
		return model.InterpolationCubicBezier
	case InterpolationCubicHermite:
		return model.InterpolationCubicHermite
	}
	return model.InterpolationStep
}

type m2LoadedTrack[T any] struct {
	InterpolationType Interpolation
	GlobalSequence    uint16
	Timestamps        [][]uint32
	Values            [][]T
}

type m2TrackBase struct {
	InterpolationType Interpolation
	GlobalSequence    uint16
	Timestamps        m2Array[m2Array[uint32]]
}

type m2Track[T any] struct {
	Base   m2TrackBase
	Values m2Array[m2Array[T]]
}

func (track m2Track[T]) Load(buf []byte, adj int) m2LoadedTrack[T] {
	timestamps := make([][]uint32, track.Base.Timestamps.Size)
	for i, ts := range track.Base.Timestamps.Load(buf, adj) {
		timestamps[i] = ts.Load(buf, adj)
	}
	values := make([][]T, track.Values.Size)
	for i, v := range track.Values.Load(buf, adj) {
		values[i] = v.Load(buf, adj)
	}

	return m2LoadedTrack[T]{track.Base.InterpolationType, track.Base.GlobalSequence, timestamps, values}
}

type C3Vector struct {
	X, Y, Z float32
}

func (vec *C3Vector) Normalize() {
	magnitude := math.Sqrt(float64(vec.X*vec.X + vec.Y*vec.Y + vec.Z*vec.Z))
	if magnitude == 0 || math.IsNaN(magnitude) || math.IsInf(magnitude, 0) {
		panic("unable to normalize vector")
	}
	vec.X = vec.X / float32(magnitude)
	vec.Y = vec.Y / float32(magnitude)
	vec.Z = vec.Z / float32(magnitude)
}

func (vec C3Vector) IntoYUp() C3Vector {
	return C3Vector{vec.X, vec.Z, -vec.Y}
}

func (vec C3Vector) IntoArray() [3]float32 {
	return [3]float32{vec.X, vec.Y, vec.Z}
}

func (vec C3Vector) IntoMGL32() mgl32.Vec3 {
	return vec.IntoArray()
}

type C2Vector struct {
	X, Y float32
}

func (vec C2Vector) IntoMGL32() mgl32.Vec2 {
	return mgl32.Vec2{vec.X, vec.Y}
}

type M2CompQuat struct {
	X, Y, Z, W int16
}

func (quat M2CompQuat) Decompress() M2F32Quat {
	decompress := func(v int16) float32 {
		if v < 0 {
			return float32(int(v)+32768) / 32767.0
		} else {
			return float32(int(v)-32767) / 32767.0
		}
		//return (float32(v) - 32767.0) / 32768.0
	}
	q := M2F32Quat{
		decompress(quat.X),
		decompress(quat.Y),
		decompress(quat.Z),
		decompress(quat.W),
	}

	if q.X == 0.0 && q.Y == 0.0 && q.Z == 0.0 && q.W == 0.0 {
		return M2F32Quat{0.0, 0.0, 0.0, 1.0}
	}
	return q
}

type M2F32Quat struct {
	X, Y, Z, W float32
}

func (quat M2F32Quat) IntoYUp() M2F32Quat {
	return M2F32Quat{quat.X, quat.Z, -quat.Y, quat.W}
}

func (quat M2F32Quat) IntoArray() [4]float32 {
	return [4]float32{quat.X, quat.Y, quat.Z, quat.W}
}

func (quat M2F32Quat) IntoMGL32() mgl32.Vec4 {
	return mgl32.Vec4{quat.X, quat.Y, quat.Z, quat.W}
}

type CAaBox struct {
	Min, Max C3Vector
}

type M2Loop struct {
	Timestamp uint32
}

type M2Sequence struct {
	ID             uint16   // Animation id in AnimationData.dbc
	VariationIndex uint16   // Sub-animation id: Which number in a row of animations this one is.
	Duration       uint32   // The length of this animation sequence in milliseconds. (BC+: was start_timestamp in vanilla/BC)
	Movespeed      float32  // This is the speed the character moves with in this animation.
	Flags          uint32   // See animation flags below.
	Frequency      int16    // This is used to determine how often the animation is played. For all animations of the same type, this adds up to 0x7FFF (32767).
	Padding        uint16   // Padding/unused
	Replay         M2Range  // May both be 0 to not repeat. Client will pick a random number of repetitions within bounds if given.
	BlendTimeIn    uint16   // The client blends (lerp) animation states between animations where the end and start values differ. This specifies how long that blending takes. Values: 0, 50, 100, 150, 200, 250, 300, 350, 500.
	BlendTimeOut   uint16   // The client blends between this sequence and the next sequence for blendTimeOut milliseconds.
	Bounds         M2Bounds // Bounding volume for this sequence
	VariationNext  int16    // id of the following animation of this AnimationID, points to an Index or is -1 if none.
	AliasNext      uint16   // id in the list of animations. Used to find actual animation if this sequence is an alias (flags & 0x40)
}

type M2Range struct {
	Minimum uint32
	Maximum uint32
}

type M2Bounds struct {
	Extent CAaBox
	Radius float32
}

type M2Vertex struct {
	Pos         C3Vector
	BoneWeights [4]uint8
	BoneIndices [4]uint8
	Normal      C3Vector
	TexCoords   [2]C2Vector
}

type M2Color struct {
	// Placeholder
}

type M2Texture struct {
	Type     uint32
	Flags    uint32
	Filename m2Array[byte]
}

type M2TextureWeight struct {
	// Placeholder
}

type M2TextureTransform struct {
	// Placeholder
}

type M2Material struct {
	// Placeholder
}

type M2Attachment struct {
	// Placeholder
}

type M2Event struct {
	// Placeholder
}

type M2Light struct {
	// Placeholder
}

type M2Camera struct {
	// Placeholder
}

type M2Ribbon struct {
	// Placeholder
}

type M2Particle struct {
	// Placeholder
}
