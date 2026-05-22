# Model Exporter

Export game models to a commonly readable format.

```
git clone --recursive https://github.com/jonathanherbst/model_export.git
go generate ./...
go run .
```

How to run the viewer, available at localhost:1234

```
cd viewer
npm run start
```

## GLTF Extension

Extension needs to support the configuration options.
- Turning on and off geosets
- Turning on and off sections of textures
- Some configuration options combine to decide if texture sections should be turned on or off
- Texture sections have different blending 

### MDLE_SegmentedTexture

A material that is built from multiple texture segments that can be overlayed on top of each other.  The segmented texture has a width and height, which defines how big the texture is is pixels.  Segments define how the texture gets composed, each segment defines a section of the texture where the segment is applied, the _x_ and _y_ fields define the top left corner where the segment is applied on the texture in pixels, and _width_ and _height_ defines the size of the segment in pixels.  Some segments overlap each other so there is a _layer_ field, which defines the order that the segments are aplied with lower numbers being applied first, and a _blend_mode_ field that defines how the segment gets blended with the already applied segments.  The configuration extension defines configurations that maps images into segments based on configuration options to build a texture.

```json
{
  "width": 2048,
  "height": 1024,
  "segments": [
    {
      "layer": 0,
      "x": 0,
      "y": 0,
      "width": 1024,
      "height": 1024,
      "blend_mode": "overlay",
    }
  ]
}
```

### MDLE_Configuration

This extension gets added to the document to define all the configuration choices for the scene and how they map to geosets and materials.  Choices have an option name which is shared between related choices.  Choices within an option are uniquely identified by the choice name and/or the color fields.  You can think of choices as an html _select_ element except option name is the label and choices are the individual options.

Elements define how choices map to geosets and materials.  Each element has a list of choice indices within the choices list, all of which should be selected by the UI for the element to be applied.  Applying the element requires enabling all the materials defined in the materials list and the mesh ids defined in the meshes list.  A material has an index into the document materials array which points to a segmented texture material, the segment field defines an index into the material's segments array, and the image field defines an index into the document's image list to be applied to the material using the segment definition.

```json
{
  "choices": [
    {
      "option": "<option_name>",
      "choice": "<choice_name>",
      "color": 0x01234567
    }
  ],
  "elements": [
    {
      "choices": [0],
      "materials": [
        {
          "material": 0,
          "segment": 2,
          "image": 50
        }
      ],
      "meshes": [0]
    }
  ]
}
```

## Resources

- [WoW list files](https://github.com/wowdev/wow-listfile/)
- [Information on file formats](https://wowdev.wiki/Main_Page)
- [CascLib documentation](http://www.zezula.net/en/casc/casclib.html)
- [Definitions for WoW database files](https://github.com/wowdev/WoWDBDefs)

## Notes

Get to the Cata+ m2 file for a race:

- Get Race ID by filtering on the `Name_lang` column of the `ChrRaces` table.
- Find model ID by finding the desired sex from the `ChrRaceXChrModel` using `ChrRaces::ID` as the foreign key
  - 0 => Male
  - 1 => Female
- Find the model from `ChrModel` from using `ChrRaceXChrModel::ChrModelID` as the id
- Find the creature display info from `CreatureDisplayInfo` using `ChrModel::DisplayID` as the id
- Find the creature model data from `CreatureModelData` using `CreatureDisplayInfo::ModelID` as the id
- The file id is `CreatureModelData::FileDataID`

Options:

- `ChrCustomizationOption` all options for a `ChrModel` linked back with `ChrCustomizationOption::ChrModelID`
- `ChrCustomizationElement` contains links to different customization types and links to `ChrCustomizationChoice` with `ChrCustomizationChoiceID`
- `ChrCustomizationChoice` all the choices for each option linked back with `ChrCustomizationOptionID`

For Textures:

From `ChrCustomizationElement::ChrCustomizationMaterialID` we can get a `ChrCustomizationMaterial` and then `ChrCustomizationMaterial::MaterialResourcesID` gets us `TextureFileData` which gets us the file id for the blp texture

With the layout id from `ChrModel::CharComponentTextureLayoutID` we can find the layer where `ChrModelTextureLayer::CharComponentTextureLayoutsID` == layout id and `ChrModelTextureLayer::ChrModelTextureTargetID_0` == `ChrCustomizationMaterial::ChrModelTextureTargetID`

Then we can find the `CharComponentTextureSections` where `CharComponentTextureSections::CharComponentTextureLayoutsID` == layout id and `((1 << CharComponentTextureSections::SectionType) & ChrModelTextureLayer::TextureSectionTypeBitMask) != 0` unless `ChrModelTextureLayer::TextureSectionTypeBitMask` is -1 in which case the texture takes up the entire layout.

The layer has the blend mode with the `BlendMode` column and the order with the `Layer` column.
