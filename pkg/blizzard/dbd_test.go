package blizzard

import (
	"testing"
)

func TestBuildVersion(t *testing.T) {
	if parseVersion("3.4.1.8622") != 848827271618990 {
		t.Errorf("parseVersion failed")
	}
	if parseVersion("3.4.1.8622.") != 848827271618990 {
		t.Errorf("parseVersion with trailing dot failed")
	}
	if parseVersion("3.4.1.") != 848827271610368 {
		t.Errorf("parseVersion partial failed")
	}
	if parseVersion("3.4.1.q") != 848827271610368 {
		t.Errorf("parseVersion with invalid char failed")
	}
	if parseVersion("3.4.1") != 848827271610368 {
		t.Errorf("parseVersion without revision failed")
	}
	if parseVersion("asdfsd") != 0 {
		t.Errorf("parseVersion invalid failed")
	}

	version := buildVersionFromString("3.4.1.8622-3.4.7.2329")
	if !version.Eql(version) {
		t.Errorf("version should equal itself")
	}
	if version.Eql(buildVersionFromString("3.4.1.8621")) {
		t.Errorf("should not equal lower")
	}
	if version.Eql(buildVersionFromString("3.4.7.2330")) {
		t.Errorf("should not equal higher")
	}
	if !version.Eql(buildVersionFromString("3.4.1.8622")) {
		t.Errorf("should equal lower bound")
	}
	if !version.Eql(buildVersionFromString("3.4.6.8622")) {
		t.Errorf("should equal middle")
	}
	if !version.Eql(buildVersionFromString("3.4.7.2329")) {
		t.Errorf("should equal upper bound")
	}
}

func TestColumnDefinition(t *testing.T) {
	col, err := columnDefFromString("int ID")
	if err != nil || col == nil || col.Name != "ID" || col.FieldType != Int || col.ForeignKey != nil || !col.Verified {
		t.Errorf("ColumnDefFromString int ID failed")
	}

	col, err = columnDefFromString("locstring MapDescription0_lang // Horde")
	if err != nil || col == nil || col.Name != "MapDescription0_lang" || col.FieldType != LocStringType || col.ForeignKey != nil || !col.Verified {
		t.Errorf("ColumnDefFromString locstring failed")
	}

	col, err = columnDefFromString("int<FileData::ID> WdtFileDataID?")
	if err != nil || col == nil || col.Name != "WdtFileDataID" || col.FieldType != Int || *col.ForeignKey != "FileData::ID" || col.Verified {
		t.Errorf("ColumnDefFromString with foreign key failed")
	}

	col, err = columnDefFromString("string InternalName")
	if err != nil || col == nil || col.Name != "InternalName" || col.FieldType != StringType || col.ForeignKey != nil || !col.Verified {
		t.Errorf("ColumnDefFromString string failed")
	}

	col, err = columnDefFromString("float Field_0_7_0_3694_007?")
	if err != nil || col == nil || col.Name != "Field_0_7_0_3694_007" || col.FieldType != FloatType || col.ForeignKey != nil || col.Verified {
		t.Errorf("ColumnDefFromString float unverified failed")
	}
}

func TestColumn(t *testing.T) {
	col, err := recordColumnDefFromString("$noninline,id$ID<32>")
	if err != nil || col == nil || col.ColumnName != "ID" || !col.Annotations.ID || !col.Annotations.NonInline || col.Annotations.Relation || col.ArrayLength != nil || col.Size == nil || col.Size.Size != 32 || col.Size.Unsigned {
		t.Errorf("RecordColumnDefFromString with annotations failed")
	}

	col, err = recordColumnDefFromString("Flags<32>[3]")
	if err != nil || col == nil || col.ColumnName != "Flags" || col.Annotations.ID || col.Annotations.NonInline || col.Annotations.Relation || *col.ArrayLength != 3 || col.Size == nil || col.Size.Size != 32 || col.Size.Unsigned {
		t.Errorf("RecordColumnDefFromString with array failed")
	}

	col, err = recordColumnDefFromString("ExpansionID<u8>")
	if err != nil || col == nil || col.ColumnName != "ExpansionID" || col.Annotations.ID || col.Annotations.NonInline || col.Annotations.Relation || col.ArrayLength != nil || col.Size == nil || col.Size.Size != 8 || !col.Size.Unsigned {
		t.Errorf("RecordColumnDefFromString unsigned failed")
	}

	col, err = recordColumnDefFromString("$id$ID<32>")
	if err != nil || col == nil || col.ColumnName != "ID" || !col.Annotations.ID || col.Annotations.NonInline || col.Annotations.Relation || col.ArrayLength != nil || col.Size == nil || col.Size.Size != 32 || col.Size.Unsigned {
		t.Errorf("RecordColumnDefFromString id only failed")
	}

	col, err = recordColumnDefFromString("Corpse[2] // a comment")
	if err != nil || col == nil || col.ColumnName != "Corpse" || col.Annotations.ID || col.Annotations.NonInline || col.Annotations.Relation || *col.ArrayLength != 2 || col.Size != nil {
		t.Errorf("RecordColumnDefFromString with comment failed")
	}
}

func TestDBDFromReader(t *testing.T) {
	// Test with example.dbd using a valid build
	selector := DBDBuildVersionSelector("12.0.1.65337")
	dbd, err := DBDFromFile("example.dbd", selector)
	if err != nil {
		t.Fatalf("DBDFromFile failed: %v", err)
	}
	if dbd == nil {
		t.Fatalf("DBD is nil")
	}
	// Basic checks
	if len(dbd.Columns) == 0 {
		t.Errorf("No columns parsed")
	}

	if dbd.IDIndex != 0 || len(dbd.Columns) != 26 || dbd.IDInlined || dbd.RelationIndex != -1 {
		t.Errorf("parsed example.dbd incorrectly")
	}
	if dbd.Columns[7].Name != "Corpse" || dbd.Columns[7].ArrayLen != 2 || dbd.Columns[7].FieldType != Float {
		t.Errorf("parsed example.dbd incorrectly")
	}
}

func TestSchemaSelector(t *testing.T) {
	buildVer := buildVersionFromString("3.4.1.46722")
	sel := DBDBuildVersionSelector("3.4.1.46722")
	if sel.buildVersion() == nil || !sel.buildVersion().Eql(buildVer) || sel.layoutHash() != nil {
		t.Errorf("NewSchemaSelectorBuild failed")
	}
	sel = DBDLayoutHashSelector(0x01234567)
	if sel.layoutHash() == nil || *sel.layoutHash() != "01234567" || sel.buildVersion() != nil {
		t.Errorf("NewSchemaSelectorLayout failed")
	}
}

func TestFieldTypeFromParts(t *testing.T) {
	ft, err := fieldTypeFromParts(Int, &fieldSize{Size: 8, Unsigned: true})
	if err != nil || ft != U8 {
		t.Errorf("FieldTypeFromParts U8 failed")
	}
	ft, err = fieldTypeFromParts(FloatType, nil)
	if err != nil || ft != Float {
		t.Errorf("FieldTypeFromParts Float failed")
	}
	ft, err = fieldTypeFromParts(Int, &fieldSize{Size: 99, Unsigned: false})
	if err == nil {
		t.Errorf("FieldTypeFromParts should fail for invalid size")
	}
}

func TestFieldTypeIsString(t *testing.T) {
	if !String.IsString() || !LocString.IsString() || U8.IsString() {
		t.Errorf("IsString failed")
	}
}
