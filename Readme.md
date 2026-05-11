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

- `CharComponentTextureLayouts` have the texture layout size from `ChrModel::CharComponentTextureLayoutID`
- `CharComponentTextureSections` has one record per customization element, linked to the model through `CharComponentTextureLayoutID`
- `ChrModelTextureLayer` links to `CharComponentTextureLayouts` and `ChrCustomizationMaterial` through `ChrModelTextureTargetID`, use the first one.
- `CharComponentTextureSections` maps with `ChrModelTextureLayer` using the `SectionType` field and the `TextureSectionTypeBitMask` field
  - if `(1 << SectionType) & TextureSectionTypeBitMask` is non zero and they map to the same layout the section is part of the layer.
  - maybe there is one layer per section?
- `ChrCustomizationMaterial` links to `TextureFileData` through `MaterialResourcesID`, which comes from `ChrCustomizationElement`


