# Blizzard Package

A golang package to handle blizzard specific files.

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
