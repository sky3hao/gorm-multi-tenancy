package plugin

import (
	"sync"

	"gorm.io/gorm"
)

const (
	defaultTenantTag = "tenant_id"
)

type TenantDBConn interface {
	CreateDBConn(tenant string, conf any) (db *gorm.DB, err error)
}

var (
	_MTPlugin *MultiTenancy
	once      sync.Once
)

type MultiTenancy struct {
	*gorm.DB

	tConn         TenantDBConn
	tenantTag     string
	dbMap         map[string]*gorm.DB
	tableMap      map[string]map[string]struct{}
	dataIsolation map[string]Model
	connConf      any
}

func Instance(connConf any) *MultiTenancy {
	if _MTPlugin == nil {
		once.Do(func() {
			_MTPlugin = &MultiTenancy{
				connConf: connConf,
			}
		})
	}
	return _MTPlugin
}

func (mt *MultiTenancy) Name() string {
	return "gorm:multi-tenancy"
}

func (mt *MultiTenancy) Initialize(db *gorm.DB) error {
	mt.DB = db
	mt.registerCallbacks(db)
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
		conn, errDB := mt.tConn.CreateDBConn(tenantId, mt.connConf)
		if errDB != nil {
			db.Error = mt.newError(errDB.Error())
			return
		}
		mt.dbMap[tenantId] = conn
		db = conn
	}
	return
}

func (mt *MultiTenancy) getTenantTag() string {
	if mt.tenantTag != "" {
		return mt.tenantTag
	}
	return defaultTenantTag
}
