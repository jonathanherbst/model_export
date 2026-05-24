package model

const SEGMENTED_TEXTURE_NAME string = "MDLE_SegmentedTexture"

type TextureSegment struct {
	X         uint   `json:"x"`
	Y         uint   `json:"y"`
	Width     uint   `json:"width"`
	Height    uint   `json:"height"`
	BlendMode string `json:"blend_mode"`
}

type SegmentedTexture struct {
	Width    uint             `json:"width"`
	Height   uint             `json:"height"`
	Segments []TextureSegment `json:"segments"`
}

const CONFIGURATION_NAME string = "MDLE_Configuration"

type ConfigExtension struct {
	Choices  []ConfigChoice  `json:"choices"`
	Elements []ConfigElement `json:"elements"`
}

type ConfigChoice struct {
	Option string `json:"option"`
	Choice string `json:"choice"`
	Color  uint32 `json:"color"`
}

type ConfigMaterial struct {
	MaterialIdx int `json:"material"`
	SegmentIdx  int `json:"segment"`
	ImageIdx    int `json:"image"`
}

type ConfigElement struct {
	ChoiceIdxes []int            `json:"choices"`
	Materials   []ConfigMaterial `json:"materials"`
	MeshIdxes   []int            `json:"meshes"`
}
