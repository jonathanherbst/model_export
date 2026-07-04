package model

const SEGMENTED_TEXTURE_NAME string = "MDLE_SegmentedTexture"

type TextureSegment struct {
	X        uint   `json:"x"`
	Y        uint   `json:"y"`
	Width    uint   `json:"width"`
	Height   uint   `json:"height"`
	Combiner string `json:"combiner"`
}

type SegmentedTexture struct {
	Width    uint             `json:"width"`
	Height   uint             `json:"height"`
	Segments []TextureSegment `json:"segments"`
}

const SCENE_NAME string = "MDLE_Scene"

type SceneExtension struct {
	Choices      []ConfigChoice      `json:"choices"`
	Elements     []ConfigElement     `json:"elements"`
	StaticMeshes []int               `json:"static_meshes"`
	Shaders      map[string]string   `json:"shaders"`
	Combiners    map[string]Combiner `json:"combiners"`
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

type Combiner struct {
	Vertex   string `json:"vertex"`
	Fragment string `json:"fragment"`
}
