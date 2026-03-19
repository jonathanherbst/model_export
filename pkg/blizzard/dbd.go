package blizzard

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

var (
	DBDErrUnknownColumn      = errors.New("unknown column")
	DBDErrInvalidIntegerSize = errors.New("invalid integer size")
	DBDErrInvalidDBDFile     = errors.New("invalid DBD file")
	DBDErrInvalidColumnDef   = errors.New("invalid column definition")
)

func DBDFromReader(r io.Reader, selector DBDSchemaSelector) (*DBDSchema, error) {
	columnDefs := make(map[string]*columnDef)
	var columns []DBDColumn
	scanner := bufio.NewScanner(r)
	state := 0 // 0 nothing, 1 column_defs, 2 builds, 3 build_select
	idIndex := 0
	idInlined := true
	var relationIndex int = -1
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if state == 3 {
				break
			} else {
				state = 0
			}
		} else if line == "COLUMNS" {
			state = 1
		} else if strings.HasPrefix(line, "COMMENT") {
			continue
		} else if strings.HasPrefix(line, "LAYOUT ") {
			state = 2
			layoutHash := strings.TrimSpace(line[7:])
			if selector.layoutHash() != nil && *selector.layoutHash() == layoutHash {
				state = 3
			}
		} else if strings.HasPrefix(line, "BUILD ") {
			if state != 3 {
				state = 2
				if selector.buildVersion() != nil {
					buildStrs := strings.Split(line[6:], ",")
					for _, bs := range buildStrs {
						bv := buildVersionFromString(strings.TrimSpace(bs))
						if selector.buildVersion().Eql(bv) {
							state = 3
						}
					}
				}
			}
		} else if state == 1 {
			if cd, err := columnDefFromString(line); err == nil && cd != nil {
				columnDefs[cd.Name] = cd
			}
		} else if state == 3 {
			if rcd, err := recordColumnDefFromString(line); err == nil && rcd != nil {
				if cd, ok := columnDefs[rcd.ColumnName]; ok {
					ft, err := fieldTypeFromParts(cd.FieldType, rcd.Size)
					if err != nil {
						return nil, err
					}
					arrayLen := 0
					if rcd.ArrayLength != nil {
						arrayLen = *rcd.ArrayLength
					}
					columns = append(columns, DBDColumn{
						Name:       rcd.ColumnName,
						FieldType:  ft,
						ArrayLen:   arrayLen,
						ForeignKey: cd.ForeignKey,
					})
					if rcd.Annotations.ID {
						idIndex = len(columns) - 1
						idInlined = !rcd.Annotations.NonInline
					}
					if rcd.Annotations.Relation {
						relationIndex = len(columns) - 1
					}
				} else {
					return nil, DBDErrUnknownColumn
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, DBDErrInvalidDBDFile
	}
	return &DBDSchema{
		IDIndex:       idIndex,
		Columns:       columns,
		IDInlined:     idInlined,
		RelationIndex: relationIndex,
	}, nil
}

func DBDFromFile(path string, selector DBDSchemaSelector) (*DBDSchema, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return DBDFromReader(f, selector)
}

func DBDBuildVersionSelector(build string) DBDSchemaSelector {
	return DBDSchemaSelector{buildVersionFromString(build)}
}

func DBDLayoutHashSelector(hash uint32) DBDSchemaSelector {
	return DBDSchemaSelector{fmt.Sprintf("%08X", hash)}
}

type DBDSchemaSelector struct {
	inner interface{}
}

func (selector DBDSchemaSelector) buildVersion() *buildVersion {
	switch v := selector.inner.(type) {
	case buildVersion:
		return &v
	default:
		return nil
	}
}

func (selector DBDSchemaSelector) layoutHash() *string {
	switch v := selector.inner.(type) {
	case string:
		return &v
	default:
		return nil
	}
}

type DBDFieldType int

const (
	U8 DBDFieldType = iota
	S8
	U16
	S16
	U32
	S32
	U64
	S64
	Float
	String
	LocString
)

func (ft DBDFieldType) IsString() bool {
	return ft == String || ft == LocString
}

func fieldTypeFromParts(cdft columnDefFieldType, size *fieldSize) (DBDFieldType, error) {
	switch cdft {
	case Int:
		if size != nil {
			switch size.Size {
			case 8:
				if size.Unsigned {
					return U8, nil
				}
				return S8, nil
			case 16:
				if size.Unsigned {
					return U16, nil
				}
				return S16, nil
			case 32:
				if size.Unsigned {
					return U32, nil
				}
				return S32, nil
			case 64:
				if size.Unsigned {
					return U64, nil
				}
				return S64, nil
			default:
				return 0, DBDErrInvalidIntegerSize
			}
		}
		return S32, nil
	case FloatType:
		return Float, nil
	case StringType:
		return String, nil
	case LocStringType:
		return LocString, nil
	}
	return 0, errors.New("invalid field type")
}

type DBDSchema struct {
	IDIndex       int
	Columns       []DBDColumn
	IDInlined     bool
	RelationIndex int
}

func (d *DBDSchema) NumColumns() int {
	return len(d.Columns) - 1
}

func (d *DBDSchema) GetIndexByName(name string) (int, error) {
	for i, col := range d.Columns {
		if col.Name == name {
			return i, nil
		}
	}
	return 0, DBDErrUnknownColumn
}

func (d *DBDSchema) GetColumn(index int) DBDColumn {
	if index < d.IDIndex {
		return d.Columns[index]
	}
	return d.Columns[index+1]
}

type DBDColumn struct {
	Name       string
	FieldType  DBDFieldType
	ArrayLen   int
	ForeignKey *string
}

type buildVersion struct {
	Lower uint64
	Upper uint64
}

func (bv buildVersion) Eql(other buildVersion) bool {
	return bv.Lower <= other.Lower && bv.Upper >= other.Upper
}

func buildVersionFromString(s string) buildVersion {
	parts := strings.Split(s, "-")
	lower := parseVersion(parts[0])
	var upper uint64
	if len(parts) > 1 {
		upper = parseVersion(parts[1])
	} else {
		upper = lower
	}
	if upper < lower {
		return buildVersion{Lower: lower, Upper: lower}
	}
	return buildVersion{Lower: lower, Upper: upper}
}

func parseVersion(s string) uint64 {
	parts := strings.Split(s, ".")
	major, _ := strconv.ParseUint(getPart(parts, 0), 10, 64)
	minor, _ := strconv.ParseUint(getPart(parts, 1), 10, 64)
	patch, _ := strconv.ParseUint(getPart(parts, 2), 10, 64)
	revision, _ := strconv.ParseUint(getPart(parts, 3), 10, 64)
	return major<<48 | minor<<40 | patch<<32 | revision
}

func getPart(parts []string, i int) string {
	if i < len(parts) {
		return parts[i]
	}
	return ""
}

type recordColumnDef struct {
	ColumnName  string
	Annotations annotations
	Size        *fieldSize
	ArrayLength *int
}

func recordColumnDefFromString(s string) (*recordColumnDef, error) {
	annotations := annotations{}
	remaining := s
	if strings.HasPrefix(remaining, "$") {
		parts := strings.SplitN(remaining[1:], "$", 2)
		if len(parts) < 1 {
			return nil, nil
		}
		annStr := parts[0]
		for _, a := range strings.Split(annStr, ",") {
			switch strings.TrimSpace(a) {
			case "id":
				annotations.ID = true
			case "noninline":
				annotations.NonInline = true
			case "relation":
				annotations.Relation = true
			}
		}
		if len(parts) > 1 {
			remaining = parts[1]
		} else {
			return nil, nil
		}
	}

	// filter out comment
	commentSplit := strings.FieldsFunc(remaining, func(r rune) bool { return r == ' ' || r == '/' })
	rest := ""
	if len(commentSplit) > 0 {
		rest = commentSplit[0]
	}

	// find array length
	var arrayLength *int
	lengthSplit := strings.SplitN(rest, "[", 2)
	nameSize := lengthSplit[0]
	if len(lengthSplit) > 1 {
		arrStr := strings.TrimSuffix(lengthSplit[1], "]")
		if val, err := strconv.Atoi(arrStr); err == nil {
			arrayLength = &val
		} else {
			return nil, nil
		}
	}

	// find field size
	var size *fieldSize
	sizeSplit := strings.SplitN(nameSize, "<", 2)
	name := sizeSplit[0]
	if len(sizeSplit) > 1 {
		sizeStr := strings.TrimSuffix(sizeSplit[1], ">")
		if strings.HasPrefix(sizeStr, "u") {
			val, err := strconv.Atoi(sizeStr[1:])
			if err != nil {
				return nil, nil
			}
			size = &fieldSize{Size: val, Unsigned: true}
		} else {
			val, err := strconv.Atoi(sizeStr)
			if err != nil {
				return nil, nil
			}
			size = &fieldSize{Size: val, Unsigned: false}
		}
	}

	if name != "" {
		return &recordColumnDef{
			ColumnName:  name,
			Annotations: annotations,
			Size:        size,
			ArrayLength: arrayLength,
		}, nil
	}
	return nil, nil
}

type annotations struct {
	ID        bool
	NonInline bool
	Relation  bool
}

type fieldSize struct {
	Size     int
	Unsigned bool
}

type columnDef struct {
	Name       string
	FieldType  columnDefFieldType
	ForeignKey *string
	Verified   bool
}

func columnDefFromString(s string) (*columnDef, error) {
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return nil, nil
	}
	fieldTypeStr := parts[0]
	var fieldType columnDefFieldType
	var foreignKey *string
	typeSplit := strings.SplitN(fieldTypeStr, "<", 2)
	switch typeSplit[0] {
	case "int":
		fieldType = Int
	case "float":
		fieldType = FloatType
	case "string":
		fieldType = StringType
	case "locstring":
		fieldType = LocStringType
	default:
		return nil, nil
	}
	if len(typeSplit) > 1 {
		fk := strings.TrimSuffix(typeSplit[1], ">")
		foreignKey = &fk
	}
	name := parts[1]
	verified := true
	if strings.HasSuffix(name, "?") {
		verified = false
		name = name[:len(name)-1]
	}
	return &columnDef{
		Name:       name,
		FieldType:  fieldType,
		ForeignKey: foreignKey,
		Verified:   verified,
	}, nil
}

type columnDefFieldType int

const (
	Int columnDefFieldType = iota
	FloatType
	StringType
	LocStringType
)
