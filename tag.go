// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-xorm/core"
)

type tagContext struct {
	engine        *Engine
	parsingTables map[reflect.Type]*core.Table

	table         *core.Table
	hasCacheTag   bool
	hasNoCacheTag bool

	col        *core.Column
	fieldValue reflect.Value
	isIndex    bool
	isUnique   bool
	indexNames map[string]int

	tagName         string
	params          []string
	preTag, nextTag string
	ignoreNext      bool
}

func splitTag(tag string) (tags []string) {
	tag = strings.TrimSpace(tag)
	var hasQuote = false
	var lastIdx = 0
	for i, t := range tag {
		if t == '\'' {
			hasQuote = !hasQuote
		} else if t == ' ' {
			if lastIdx < i && !hasQuote {
				tags = append(tags, strings.TrimSpace(tag[lastIdx:i]))
				lastIdx = i + 1
			}
		}
	}
	if lastIdx < len(tag) {
		tags = append(tags, strings.TrimSpace(tag[lastIdx:]))
	}
	return
}

// tagHandler describes tag handler for XORM
type tagHandler func(ctx *tagContext) error

var (
	// defaultTagHandlers enumerates all the default tag handler
	defaultTagHandlers = map[string]tagHandler{
		"<-":         OnlyFromDBTagHandler,
		"->":         OnlyToDBTagHandler,
		"PK":         PKTagHandler,
		"NULL":       NULLTagHandler,
		"NOT":        IgnoreTagHandler,
		"AUTOINCR":   AutoIncrTagHandler,
		"DEFAULT":    DefaultTagHandler,
		"CREATED":    CreatedTagHandler,
		"UPDATED":    UpdatedTagHandler,
		"DELETED":    DeletedTagHandler,
		"VERSION":    VersionTagHandler,
		"UTC":        UTCTagHandler,
		"LOCAL":      LocalTagHandler,
		"NOTNULL":    NotNullTagHandler,
		"INDEX":      IndexTagHandler,
		"UNIQUE":     UniqueTagHandler,
		"CACHE":      CacheTagHandler,
		"NOCACHE":    NoCacheTagHandler,
		"BELONGS_TO": BelongsToTagHandler,
	}
)

func init() {
	for k := range core.SqlTypes {
		defaultTagHandlers[k] = SQLTypeTagHandler
	}
}

// IgnoreTagHandler describes ignored tag handler
func IgnoreTagHandler(ctx *tagContext) error {
	return nil
}

// OnlyFromDBTagHandler describes mapping direction tag handler
func OnlyFromDBTagHandler(ctx *tagContext) error {
	ctx.col.MapType = core.ONLYFROMDB
	return nil
}

// OnlyToDBTagHandler describes mapping direction tag handler
func OnlyToDBTagHandler(ctx *tagContext) error {
	ctx.col.MapType = core.ONLYTODB
	return nil
}

// PKTagHandler decribes primary key tag handler
func PKTagHandler(ctx *tagContext) error {
	ctx.col.IsPrimaryKey = true
	ctx.col.Nullable = false
	return nil
}

// NULLTagHandler describes null tag handler
func NULLTagHandler(ctx *tagContext) error {
	ctx.col.Nullable = (strings.ToUpper(ctx.preTag) != "NOT")
	return nil
}

// NotNullTagHandler describes notnull tag handler
func NotNullTagHandler(ctx *tagContext) error {
	ctx.col.Nullable = false
	return nil
}

// AutoIncrTagHandler describes autoincr tag handler
func AutoIncrTagHandler(ctx *tagContext) error {
	ctx.col.IsAutoIncrement = true
	/*
		if len(ctx.params) > 0 {
			autoStartInt, err := strconv.Atoi(ctx.params[0])
			if err != nil {
				return err
			}
			ctx.col.AutoIncrStart = autoStartInt
		} else {
			ctx.col.AutoIncrStart = 1
		}
	*/
	return nil
}

// DefaultTagHandler describes default tag handler
func DefaultTagHandler(ctx *tagContext) error {
	if len(ctx.params) > 0 {
		ctx.col.Default = ctx.params[0]
	} else {
		ctx.col.Default = ctx.nextTag
		ctx.ignoreNext = true
	}
	return nil
}

// CreatedTagHandler describes created tag handler
func CreatedTagHandler(ctx *tagContext) error {
	ctx.col.IsCreated = true
	return nil
}

// VersionTagHandler describes version tag handler
func VersionTagHandler(ctx *tagContext) error {
	ctx.col.IsVersion = true
	ctx.col.Default = "1"
	return nil
}

// UTCTagHandler describes utc tag handler
func UTCTagHandler(ctx *tagContext) error {
	ctx.col.TimeZone = time.UTC
	return nil
}

// LocalTagHandler describes local tag handler
func LocalTagHandler(ctx *tagContext) error {
	if len(ctx.params) == 0 {
		ctx.col.TimeZone = time.Local
	} else {
		var err error
		ctx.col.TimeZone, err = time.LoadLocation(ctx.params[0])
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdatedTagHandler describes updated tag handler
func UpdatedTagHandler(ctx *tagContext) error {
	ctx.col.IsUpdated = true
	return nil
}

// DeletedTagHandler describes deleted tag handler
func DeletedTagHandler(ctx *tagContext) error {
	ctx.col.IsDeleted = true
	return nil
}

// IndexTagHandler describes index tag handler
func IndexTagHandler(ctx *tagContext) error {
	if len(ctx.params) > 0 {
		ctx.indexNames[ctx.params[0]] = core.IndexType
	} else {
		ctx.isIndex = true
	}
	return nil
}

// UniqueTagHandler describes unique tag handler
func UniqueTagHandler(ctx *tagContext) error {
	if len(ctx.params) > 0 {
		ctx.indexNames[ctx.params[0]] = core.UniqueType
	} else {
		ctx.isUnique = true
	}
	return nil
}

// SQLTypeTagHandler describes SQL Type tag handler
func SQLTypeTagHandler(ctx *tagContext) error {
	ctx.col.SQLType = core.SQLType{Name: ctx.tagName}
	if len(ctx.params) > 0 {
		if ctx.tagName == core.Enum {
			ctx.col.EnumOptions = make(map[string]int)
			for k, v := range ctx.params {
				v = strings.TrimSpace(v)
				v = strings.Trim(v, "'")
				ctx.col.EnumOptions[v] = k
			}
		} else if ctx.tagName == core.Set {
			ctx.col.SetOptions = make(map[string]int)
			for k, v := range ctx.params {
				v = strings.TrimSpace(v)
				v = strings.Trim(v, "'")
				ctx.col.SetOptions[v] = k
			}
		} else {
			var err error
			if len(ctx.params) == 2 {
				ctx.col.Length, err = strconv.Atoi(ctx.params[0])
				if err != nil {
					return err
				}
				ctx.col.Length2, err = strconv.Atoi(ctx.params[1])
				if err != nil {
					return err
				}
			} else if len(ctx.params) == 1 {
				ctx.col.Length, err = strconv.Atoi(ctx.params[0])
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ExtendsTagHandler describes extends tag handler
func ExtendsTagHandler(ctx *tagContext) error {
	var fieldValue = ctx.fieldValue
	switch fieldValue.Kind() {
	case reflect.Ptr:
		f := fieldValue.Type().Elem()
		if f.Kind() == reflect.Struct {
			fieldPtr := fieldValue
			fieldValue = fieldValue.Elem()
			if !fieldValue.IsValid() || fieldPtr.IsNil() {
				fieldValue = reflect.New(f).Elem()
			}
		}
		fallthrough
	case reflect.Struct:
		parentTable, err := ctx.engine.mapType(ctx.parsingTables, fieldValue)
		if err != nil {
			return err
		}
		for _, col := range parentTable.Columns() {
			col.FieldName = fmt.Sprintf("%v.%v", ctx.col.FieldName, col.FieldName)
			ctx.table.AddColumn(col)
			for indexName, indexType := range col.Indexes {
				addIndex(indexName, ctx.table, col, indexType)
			}
		}
	default:
		//TODO: warning
	}
	return nil
}

// CacheTagHandler describes cache tag handler
func CacheTagHandler(ctx *tagContext) error {
	if !ctx.hasCacheTag {
		ctx.hasCacheTag = true
	}
	return nil
}

// NoCacheTagHandler describes nocache tag handler
func NoCacheTagHandler(ctx *tagContext) error {
	if !ctx.hasNoCacheTag {
		ctx.hasNoCacheTag = true
	}
	return nil
}

// BelongsToTagHandler describes belongs_to tag handler
func BelongsToTagHandler(ctx *tagContext) error {
	if !isStruct(ctx.fieldValue.Type()) {
		return errors.New("Tag belongs_to cannot be applied on non-struct field")
	}

	ctx.col.AssociateType = core.AssociateBelongsTo
	var t reflect.Value
	if ctx.fieldValue.Kind() == reflect.Struct {
		t = ctx.fieldValue
	} else {
		if ctx.fieldValue.Type().Kind() == reflect.Ptr && ctx.fieldValue.Type().Elem().Kind() == reflect.Struct {
			if ctx.fieldValue.IsNil() {
				t = reflect.New(ctx.fieldValue.Type().Elem()).Elem()
			} else {
				t = ctx.fieldValue
			}
		} else {
			return errors.New("Only struct or ptr to struct field could add belongs_to flag")
		}
	}

	belongsT, err := ctx.engine.mapType(ctx.parsingTables, t)
	if err != nil {
		return err
	}
	pks := belongsT.PKColumns()
	if len(pks) != 1 {
		panic("unsupported non or composited primary key cascade")
		return errors.New("blongs_to only should be as a tag of table has one primary key")
	}

	ctx.col.AssociateTable = belongsT
	ctx.col.SQLType = pks[0].SQLType

	if len(ctx.col.Name) == 0 {
		ctx.col.Name = ctx.engine.ColumnMapper.Obj2Table(ctx.col.FieldName) + "_id"
	}
	return nil
}
