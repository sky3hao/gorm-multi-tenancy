package plugin

import (
	"errors"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

type TenantDBConn interface {
	CreateDBConn(tenant string) (db *gorm.DB, err error)
}

var (
	_MTPlugin *MultiTenancy
	once      sync.Once
)

type MultiTenancy struct {
	*gorm.DB

	tConn     TenantDBConn
	tenantTag string
	dbMap     map[string]*gorm.DB
	tableMap  map[string]map[string]struct{}
}

func Instance() *MultiTenancy {
	if _MTPlugin == nil {
		once.Do(func() {
			_MTPlugin = &MultiTenancy{}
		})
	}
	return _MTPlugin
}

func (mt *MultiTenancy) Name() string {
	return "gorm:multi-tenancy"
}

func (mt *MultiTenancy) Initialize(db *gorm.DB) error {
	mt.DB = db
	return nil
}

func (mt *MultiTenancy) Register(tenantTag string, conn TenantDBConn) *MultiTenancy {
	mt.dbMap = make(map[string]*gorm.DB)
	mt.tConn = conn
	mt.tenantTag = tenantTag

	_MTPlugin = mt
	return mt
}

func (mt *MultiTenancy) AddDB(tenantId string, db *gorm.DB) {
	if mt.dbMap == nil {
		mt.dbMap = make(map[string]*gorm.DB)
	}
	mt.dbMap[tenantId] = db
	return
}

func (mt *MultiTenancy) GetDBByTenantId(tenantId string) (db *gorm.DB, err error) {
	if mt.dbMap == nil {
		mt.dbMap = make(map[string]*gorm.DB)
	}
	var ok bool
	db, ok = mt.dbMap[tenantId]
	if !ok {
		conn, errDB := mt.tConn.CreateDBConn(tenantId)
		if errDB != nil {
			db.Error = mt.newError(errDB.Error())
			return db, errDB
		}
		mt.dbMap[tenantId] = conn
		db = conn
	}
	return
}

func (mt *MultiTenancy) newError(errorStr string) (err error) {
	return errors.New(fmt.Sprintf("【gorm:multi-tenancy】%s", errorStr))
}
