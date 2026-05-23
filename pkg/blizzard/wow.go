package blizzard

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"jph/model-export/pkg/model"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/google/go-github/v84/github"
)

func IsWOWCasc(casc *Casc) bool {
	return strings.HasPrefix(casc.ProductName, "wow")
}

func OpenWOWCasc(casc *Casc, cachePath string) (*WOWCasc, error) {
	if !IsWOWCasc(casc) {
		panic("attemted to open a wow casc that isn's a wow casc")
	}

	listfilePath, err := WOWGetLatestListfile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open wow casc: %w", err)
	}
	casc.ListFilePath = &listfilePath

	dbdPath, err := WOWGetLatestDBD(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open wow casc: %w", err)
	}

	tables := make(map[string]string)
	for file_data := range func(yield func(FileData) bool) { casc.SearchFiles("*.db2", yield) } {
		sanitized_name := strings.ReplaceAll(file_data.Name, "\\", "/")
		table_name := strings.TrimSuffix(path.Base(sanitized_name), ".db2")
		tables[table_name] = file_data.Name
	}

	zipReader, err := zip.OpenReader(dbdPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open dbd zip: %w", err)
	}

	return &WOWCasc{casc, zipReader, tables}, nil
}

type WOWCasc struct {
	Casc   *Casc
	dbdZip *zip.ReadCloser
	tables map[string]string
}

func (wow *WOWCasc) Close() {
	wow.Casc.Close()
	wow.dbdZip.Close()
}

func (wow *WOWCasc) GetTables(yield func(string) bool) {
	for k := range wow.tables {
		if !yield(k) {
			return
		}
	}
}

func (wow *WOWCasc) GetTable(name string) (*DBDTable, error) {
	file, err := wow.Casc.OpenFileByName(wow.tables[name], true)
	if err != nil {
		return nil, fmt.Errorf("get table file: %w", err)
	}
	db2, err := OpenDB2File(file)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("get table, open db2: %w", err)
	}
	dbdName := name + ".dbd"
	dbd, err := wow.dbdZip.Open(dbdName)
	if err != nil {
		db2.Close()
		return nil, fmt.Errorf("get table, open dbd: %w", err)
	}
	defer dbd.Close()

	table, err := DBDTableFromReader(dbd, db2)
	if err != nil {
		db2.Close()
		return nil, fmt.Errorf("get table, open table: %w", err)
	}
	return table, nil
}

func (wow *WOWCasc) FindRaceModelId(modelName string, gender *int) *int {
	var raceId int = -1
	if races, err := wow.GetTable("ChrRaces"); err == nil {
		for record := range races.GetRecords {
			if record.GetStringFieldByName("Name_lang") == modelName {
				raceId = int(record.GetID())
				break
			}
		}
	}
	if raceId < 0 {
		return nil
	}

	var modelId int = -1
	if chrxmodels, err := wow.GetTable("ChrRaceXChrModel"); err == nil {
		for record := range chrxmodels.GetFixedRecordsByForeignKey(uint32(raceId)) {
			if gender != nil && int(record.GetIntFieldByName("Sex")) == *gender {
				modelId = int(record.GetIntFieldByName("ChrModelID"))
				break
			}
		}
	}

	if modelId < 0 {
		return nil
	}
	return &modelId
}

func (wow *WOWCasc) LoadModelFromId(modelId int) *model.Model {
	modelTable, err := wow.GetTable("ChrModel")
	if err != nil {
		panic("no ChrModel table")
	}
	chrDisplayInfo, err := wow.GetTable("CreatureDisplayInfo")
	if err != nil {
		panic("no CreatureDisplayInfo table")
	}
	creatureModelData, err := wow.GetTable("CreatureModelData")
	if err != nil {
		panic("no CreatureModelData table")
	}

	modelRecord := modelTable.GetFixedRecordById(uint32(modelId))
	if modelRecord == nil {
		return nil
	}
	var fileDataId int = -1
	displayId := modelRecord.GetIntFieldByName("DisplayID")
	if displayInfo := chrDisplayInfo.GetFixedRecordById(uint32(displayId)); displayInfo != nil {
		modelDataId := displayInfo.GetIntFieldByName("ModelID")
		if record := creatureModelData.GetFixedRecordById(uint32(modelDataId)); record != nil {
			fileDataId = int(record.GetIntFieldByName("FileDataID"))
		}
	}
	if fileDataId < 0 {
		return nil
	}

	var mdl model.Model

	modelFile, err := wow.Casc.OpenFileById(uint32(fileDataId), false)
	if err != nil {
		return nil
	}

	m2File, err := M2FromReader(modelFile)
	if err != nil {
		return nil
	}
	m2File.FillModel(&mdl)

	if len(m2File.SkinFileIds) > 0 {
		skinFile, err := wow.Casc.OpenFileById(uint32(m2File.SkinFileIds[0]), false)
		if err != nil {
			return nil
		}
		buf, err := io.ReadAll(skinFile)
		if err != nil {
			return nil
		}
		skin, err := M2SkinFromBuf(buf)
		if err != nil {
			return nil
		}
		skin.FillModel(&mdl, *m2File)
	}

	for _, skelFileId := range m2File.SkelFileIds {
		skelFile, err := wow.Casc.OpenFileById(uint32(skelFileId), false)
		if err != nil {
			return nil
		}
		skel, err := M2SkelFromReader(skelFile)
		if err != nil {
			return nil
		}
		skel.FillModel(&mdl)
	}

	wow.loadConfigurationOptions(&mdl, *modelRecord)

	return &mdl
}

func (wow *WOWCasc) loadConfigurationOptions(mdl *model.Model, modelRecord DBDRecord) {
	custOptionTable, err := wow.GetTable("ChrCustomizationOption")
	if err != nil {
		panic("no ChrCustomizationOption table")
	}
	custChoicesTable, err := wow.GetTable("ChrCustomizationChoice")
	if err != nil {
		panic("no ChrCustomizationChoice table")
	}
	custChoicesCache := custChoicesTable.Cache()
	custElementTable, err := wow.GetTable("ChrCustomizationElement")
	if err != nil {
		panic("no ChrCustomizationElement table")
	}
	custMaterialTable, err := wow.GetTable("ChrCustomizationMaterial")
	if err != nil {
		panic("no ChrCustomizationMaterial table")
	}
	custMaterialCache := custMaterialTable.Cache()
	textureFileDataTable, err := wow.GetTable("TextureFileData")
	if err != nil {
		panic("no TextureFileData table")
	}
	textureFileDataCache := textureFileDataTable.Cache()
	textureSectionTable, err := wow.GetTable("CharComponentTextureSections")
	if err != nil {
		panic("no CharComponentTextureSections table")
	}
	textureLayerTable, err := wow.GetTable("ChrModelTextureLayer")
	if err != nil {
		panic("no ChrModelTextureLayer table")
	}
	chrModelMaterialTable, err := wow.GetTable("ChrModelMaterial")
	if err != nil {
		panic("no ChrModelMaterial table")
	}
	textureLayoutsTable, err := wow.GetTable("CharComponentTextureLayouts")
	if err != nil {
		panic("no CharComponentTextureLayouts table")
	}
	chrCustGeosetTable, err := wow.GetTable("ChrCustomizationGeoset")
	if err != nil {
		panic("no ChrCustomizationGeoset table")
	}
	chrCustGeosetCache := chrCustGeosetTable.Cache()

	layoutId := uint32(modelRecord.GetIntFieldByName("CharComponentTextureLayoutID"))
	layout := textureLayoutsTable.GetFixedRecordById(layoutId)
	if layout == nil {
		panic("model has unknown layout id")
	}

	mdl.Choices = make([]model.ConfigChoice, 0)
	choices := make(map[uint32]int)
	for option := range custOptionTable.GetFixedRecordsByForeignKey(modelRecord.GetID()) {
		optionName := option.GetStringFieldByName("Name_lang")

		choiceRecords := make([]DBDRecord, 0)
		for choice := range custChoicesCache.GetFixedRecordsByForeignKey(option.GetID()) {
			choiceRecords = append(choiceRecords, choice)
		}
		sort.Slice(choiceRecords, func(i, j int) bool {
			return choiceRecords[i].GetIntFieldByName("OrderIndex") < choiceRecords[j].GetIntFieldByName("OrderIndex")
		})

		for _, choice := range choiceRecords {
			choices[choice.GetID()] = len(mdl.Choices)
			mdl.Choices = append(mdl.Choices, model.ConfigChoice{
				Option: optionName,
				Choice: choice.GetStringFieldByName("Name_lang"),
				Color:  uint32(GetSliceFieldByName[int64](choice, "SwatchColor")[0]),
			})
		}
	}

	// cache all the model materials
	var modelMaterials []DBDRecord
	mdl.SegmentedTextures = make([]model.SegmentedTexture, 0)
	for modelMaterial := range chrModelMaterialTable.GetFixedRecordsByForeignKey(layoutId) {
		modelMaterials = append(modelMaterials, modelMaterial)
		textureType := modelMaterial.GetIntFieldByName("TextureType")
		mdl.SegmentedTextures = append(mdl.SegmentedTextures, model.SegmentedTexture{
			Name:     textureTypeNames[uint32(textureType)],
			Width:    uint(modelMaterial.GetIntFieldByName("Width")),
			Height:   uint(modelMaterial.GetIntFieldByName("Height")),
			Segments: make([]model.TextureSegment, 0),
		})
	}

	// cache all the texture sections for the layout
	var textureSections []DBDRecord
	for textureSection := range textureSectionTable.GetFixedRecordsByForeignKey(layoutId) {
		textureSections = append(textureSections, textureSection)
	}

	// cache all the texture layers and sort them by the layer field
	var textureLayers []DBDRecord
	for textureLayer := range textureLayerTable.GetFixedRecordsByForeignKey(layoutId) {
		textureLayers = append(textureLayers, textureLayer)
	}
	sort.Slice(textureLayers, func(i, j int) bool {
		return textureLayers[i].GetIntFieldByName("Layer") < textureLayers[j].GetIntFieldByName("Layer")
	})

	// setup all the material segments
	layerMapping := make(map[int64][]int)
	for _, textureLayer := range textureLayers {
		textureType := textureLayer.GetIntFieldByName("TextureType")
		materialIdx := slices.IndexFunc(modelMaterials, func(mat DBDRecord) bool { return mat.GetIntFieldByName("TextureType") == textureType })
		material := &mdl.SegmentedTextures[materialIdx]

		textureTargetId := GetSliceFieldByName[int64](textureLayer, "ChrModelTextureTargetID")[0]
		layerMapping[textureTargetId] = []int{materialIdx, len(material.Segments)}

		sectionTypeBitmask := textureLayer.GetIntFieldByName("TextureSectionTypeBitMask")
		if sectionTypeBitmask == -1 {
			material.Segments = append(material.Segments, model.TextureSegment{
				X:         0,
				Y:         0,
				Width:     material.Width,
				Height:    material.Height,
				BlendMode: "overlay", // set them all to overlay for now
			})
		} else {
			for _, section := range textureSections {
				sectionType := section.GetIntFieldByName("SectionType")
				if (1<<sectionType)&sectionTypeBitmask != 0 {
					material.Segments = append(material.Segments, model.TextureSegment{
						X:         uint(section.GetIntFieldByName("X")),
						Y:         uint(section.GetIntFieldByName("Y")),
						Width:     uint(section.GetIntFieldByName("Width")),
						Height:    uint(section.GetIntFieldByName("Height")),
						BlendMode: "overlay", // set them all to overlay for now
					})
				}
			}
		}
	}

	imageMap := make(map[uint32]int)

	for custElement := range custElementTable.GetRecords {
		choiceId := uint32(custElement.GetIntFieldByName("ChrCustomizationChoiceID"))
		if choiceIdx, ok := choices[choiceId]; ok {
			component := model.ConfigElement{
				ChoiceIdxes: []int{choiceIdx},
				Materials:   make([]model.ConfigMaterial, 0),
			}

			relatedChoiceId := uint32(custElement.GetIntFieldByName("RelatedChrCustomizationChoiceID"))
			if choiceIdx, ok := choices[relatedChoiceId]; ok {
				component.ChoiceIdxes = append(component.ChoiceIdxes, choiceIdx)
			}

			geosetId := int(custElement.GetIntFieldByName("ChrCustomizationGeosetID"))
			if geosetId > 0 {
				if geoset := chrCustGeosetCache.GetFixedRecordById(uint32(geosetId)); geoset != nil {
					geosetType := geoset.GetIntFieldByName("GeosetType")
					geosetSubId := geoset.GetIntFieldByName("GeosetID")
					meshName := GetMeshName(uint16(geosetType*100 + geosetSubId))
					meshIdx := slices.IndexFunc(mdl.Skin.Meshes, func(mesh model.Mesh) bool { return mesh.Name == meshName })
					if meshIdx != -1 {
						component.MeshIdxes = []int{meshIdx}
					}
				} else {
					panic("geoset not found")
				}
			}

			materialId := uint32(custElement.GetIntFieldByName("ChrCustomizationMaterialID"))
			material := custMaterialCache.GetFixedRecordById(materialId)
			if material != nil {
				textureTargetId := material.GetIntFieldByName("ChrModelTextureTargetID")
				matInfo, ok := layerMapping[textureTargetId]
				if !ok {
					panic("configuration references a textureTargetId we don't know about")
				}
				textureSegment := model.ConfigMaterial{
					MaterialIdx: matInfo[0],
					SegmentIdx:  matInfo[1],
				}

				resourcesId := material.GetIntFieldByName("MaterialResourcesID")
				for textureFileData := range textureFileDataCache.GetFixedRecordsByForeignKey(uint32(resourcesId)) {
					textureFileId := textureFileData.GetID()
					if idx, ok := imageMap[textureFileId]; ok {
						textureSegment.ImageIdx = idx
						break
					} else {
						// what do I do with multiple texture file datas?
						if textureFile, err := wow.Casc.OpenFileById(textureFileData.GetID(), false); err == nil {
							if blp, err := BLPFromReader(textureFile); err == nil {
								if img, err := blp.Decode(0); err == nil {
									imgIdx := len(mdl.Images)
									mdl.Images = append(mdl.Images, img)
									imageMap[textureFileId] = imgIdx
									textureSegment.ImageIdx = imgIdx
									break
								}
							}
						}
					}
				}
				component.Materials = append(component.Materials, textureSegment)
			}
			mdl.Elements = append(mdl.Elements, component)
		}
	}
}

func WOWGetLatestListfile(cachePath string) (string, error) {
	destPath := filepath.Join(cachePath, "wow-listfile.csv")
	if err := downloadLatestGHRelease("wowdev", "wow-listfile", "verified-listfile-withcapitals.csv", destPath); err != nil {
		return "", err
	}
	return destPath, nil
}

func WOWGetLatestDBD(cachePath string) (string, error) {
	destPath := filepath.Join(cachePath, "wow-dbd.zip")
	if err := downloadLatestGHRelease("wowdev", "WoWDBDefs", "dbd.zip", destPath); err != nil {
		return "", err
	}
	return destPath, nil
}

func downloadLatestGHRelease(owner, repo, assetName, destPath string) error {
	ctx := context.Background()
	client := github.NewClient(nil)
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return err
	}
	var assetURL string
	var expectedSHA256 string
	for _, asset := range release.Assets {
		if *asset.Name == assetName {
			assetURL = *asset.BrowserDownloadURL
			if asset.Digest != nil && strings.HasPrefix(*asset.Digest, "sha256:") {
				expectedSHA256 = (*asset.Digest)[7:]
			}
			break
		}
	}
	if assetURL == "" {
		return fmt.Errorf("asset %s not found in latest release", assetName)
	}

	// Check if file exists and SHA256 matches
	if expectedSHA256 != "" {
		actualSHA256, err := computeFileSHA256(destPath)
		if err == nil && actualSHA256 == expectedSHA256 {
			return nil // Already have the correct file
		}
	}

	resp, err := http.Get(assetURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed: %s", resp.Status)
	}
	file, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	return err
}

func computeFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}
