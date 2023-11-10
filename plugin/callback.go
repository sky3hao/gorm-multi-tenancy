package plugin

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/utils"
)

type Model interface {
	TableName() string
	DataIsolation() bool
	AutoMigrate(db *gorm.DB, tableName string) error
}

func (mt *MultiTenancy) registerCallbacks(db *gorm.DB) {
	mt.Callback().Create().Before("*").Register("gorm:multi-tenancy", mt.createBeforeCallback)
	mt.Callback().Query().Before("*").Register("gorm:multi-tenancy", mt.queryBeforeCallback)
	mt.Callback().Update().Before("*").Register("gorm:multi-tenancy", mt.updateBeforeCallback)
	mt.Callback().Delete().Before("*").Register("gorm:multi-tenancy", mt.deleteBeforeCallback)
	mt.Callback().Row().Before("*").Register("gorm:multi-tenancy", mt.rowBeforeCallback)
	mt.Callback().Raw().Before("*").Register("gorm:multi-tenancy", mt.rawBeforeCallback)
}

func (mt *MultiTenancy) createBeforeCallback(db *gorm.DB) {
	mt.commonCallback(db, mt.getTenantIdByModel)
}

func (mt *MultiTenancy) queryBeforeCallback(db *gorm.DB) {
	mt.commonCallback(db, mt.getTenantIdBySql)
}

func (mt *MultiTenancy) commonCallback(db *gorm.DB, getTenantId func(db *gorm.DB) (tenantId string)) {
	if db.Error != nil {
		return
	}
	model, ok := mt.DataIsolation(db)
	if !ok {
		return
	}

	tenantId := getTenantId(db)
	if db.Error != nil {
		return
	}

	mt.getAndSwitchDBConnPool(db, tenantId)
	if db.Error != nil {
		return
	}
	mt.AutoMigrate(db, tenantId, model)

}

func (mt *MultiTenancy) AutoMigrate(db *gorm.DB, tenantId string, model Model) {
	if mt.tableMap == nil {
		mt.tableMap = make(map[string]map[string]struct{})
	}
	_, ok := mt.tableMap[tenantId]
	if !ok {
		mt.tableMap[tenantId] = make(map[string]struct{})
	}
	table := db.Statement.Table
	_, ok = mt.tableMap[tenantId][table]
	if ok {
		// 不需要建表
		return
	}

	createDB, _ := mt.GetDBByTenantId(tenantId)
	db.Error = model.AutoMigrate(createDB, db.Statement.Table)
}

func (mt *MultiTenancy) updateBeforeCallback(db *gorm.DB) {
	mt.commonCallback(db, mt.getTenantIdBySql)
}

func (mt *MultiTenancy) deleteBeforeCallback(db *gorm.DB) {
	mt.commonCallback(db, mt.getTenantIdBySql)
}

func (mt *MultiTenancy) rowBeforeCallback(db *gorm.DB) {}

func (mt *MultiTenancy) rawBeforeCallback(db *gorm.DB) {}

func (mt *MultiTenancy) SetDataIsolation(model ...Model) (err error) {
	if mt.dataIsolation == nil {
		mt.dataIsolation = make(map[string]Model)
	}
	for _, m := range model {
		mt.dataIsolation[m.TableName()] = m
	}
	return
}

func (mt *MultiTenancy) DataIsolation(db *gorm.DB) (model Model, dataIsolation bool) {
	var ok bool
	model, ok = mt.dataIsolation[db.Statement.Table]
	if ok {
		dataIsolation = model.DataIsolation()
		return
	}
	model, ok = mt.dataIsolation[db.Statement.Schema.Table]
	if ok {
		dataIsolation = model.DataIsolation()
		return
	}
	return
}

func (mt *MultiTenancy) newError(errorStr string) (err error) {
	return errors.New(fmt.Sprintf("【gorm:multi-tenancy】%s", errorStr))
}

func (mt *MultiTenancy) getAndSwitchDBConnPool(db *gorm.DB, tenantId string) {
	if tenantId == "" {
		db.Error = mt.newError("no tenant ID detected")
		return
	}

	_, ok := mt.dbMap[tenantId]
	if !ok {
		conn, errDB := mt.tConn.CreateDBConn(tenantId)
		if errDB != nil {
			db.Error = mt.newError(errDB.Error())
			return
		}
		mt.dbMap[tenantId] = conn
	}
	db.Statement.ConnPool = mt.dbMap[tenantId].ConnPool
	return
}

func (mt *MultiTenancy) getTenantIdBySql(db *gorm.DB) (tenantId string) {
	defer func() { tenantId = strings.TrimSpace(tenantId) }()
	whereClauses, ok := db.Statement.Clauses["WHERE"].Expression.(clause.Where)
	if !ok {
		db.Error = mt.newError("no WHERE clause detected")
		return
	}
	var build strings.Builder
	for i, expr := range whereClauses.Exprs {
		v := expr.(clause.Expr)
		sql := v.SQL
		for _, vi := range v.Vars {
			sql = strings.Replace(sql, "?", utils.ToString(vi), 1)
		}
		build.WriteString(sql)
		if i != len(whereClauses.Exprs)-1 {
			build.WriteString(" AND ")
		}
	}
	sql := build.String()
	strs := strings.Split(sql, "AND")
	for _, stri := range strs {
		sqlField := strings.Split(stri, "=")
		if strings.TrimSpace(sqlField[0]) == mt.getTenantTag() {
			tenantId = sqlField[1]
			return
		}
	}
	return
}

func (mt *MultiTenancy) getTenantIdByModel(db *gorm.DB) (tenantId string) {
	defer func() { tenantId = strings.TrimSpace(tenantId) }()
	var ok bool
	if db.Statement.Schema != nil {
		for _, field := range db.Statement.Schema.Fields {
			if field.DBName != mt.getTenantTag() {
				continue
			}
			switch db.Statement.ReflectValue.Kind() {
			case reflect.Slice, reflect.Array:
				var tenantIdi string
				for i := 0; i < db.Statement.ReflectValue.Len(); i++ {
					fieldValue, isZero := field.ValueOf(db.Statement.ReflectValue.Index(i))
					if isZero {
						continue
					}
					tenantIdi, ok = fieldValue.(string)
					if !ok {
						db.Error = mt.newError("assertion failed, only string field sublibrary is supported")
						return
					}
					if tenantId != "" && tenantIdi != tenantId {
						db.Error = mt.newError("batch inserts to different databases are not supported")
						return
					}
					tenantId = tenantIdi
				}
				return
			case reflect.Struct:
				fieldValue, isZero := field.ValueOf(db.Statement.ReflectValue)
				if isZero {
					return
				}
				tenantId, ok = fieldValue.(string)
				if !ok {
					db.Error = mt.newError("assertion failed, only string field sublibrary is supported")
					return
				}
			}
		}
	}
	return
}
