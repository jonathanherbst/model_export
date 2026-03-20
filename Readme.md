# Model Exporter

Export game models to a commonly readable format.

Updating casclib

``` sh
zig fetch --save git+https://github.com/jonathanherbst/CascLib.git#zig
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
