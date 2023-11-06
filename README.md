# gorm-multi-tenancy

## Introduce

Dynamically switch database connections based on identifiers

## Install

```bash
go get -u github.com/sky3hao/gorm-multi-tenancy
```

## Useage

### Implement `TenantDBConn` interface

```go
type TenantConn struct{}

func (c TenantConn) CreateDBConn(tenant string, connConf any) (db *gorm.DB, err error) {
    if confData, ok := connConf.(*conf.Data); ok {
        db, err = gorm.Open(mysql.Open(fmt.Sprintf(confData.Database.Dsn, tenant)), &gorm.Config{})
        if err != nil {
            return
        }
        sqlDB, err := db.DB()
        if err != nil {
            sqlDB.Close()
            return
        }
        sqlDB.SetMaxIdleConns(int(confData.Database.MaxIdle))
        sqlDB.SetMaxOpenConns(int(confData.Database.MaxOpen))
        sqlDB.SetConnMaxLifetime(confData.Database.ConnLifetime.AsDuration())
    }
    
    return
}
```

## Register 

```go
// Master db connction 
db, err = gorm.Open(mysql.Open(conf.Database.Source), gormConfig(conf, logger))
if err != nil {
	// ...
}

// Register to db 
mt := plugin.Instance(conf)
mt.Register("tenant_tag", TenantConn{})
err = db.Use(mt)
if err != nil {
    // ... 
}
```

## Dynamic db connection switch

If you use grpc to pass context, you can use `metadata` to get the `tenant_key`

```go
func (d *Data) DB(ctx context.Context) *gorm.DB {
    md, _ := metadata.FromIncomingContext(ctx)
    mdValue := md.Get("tenant_key")
    if len(mdValue) < 1 {
        painc("missing tenant key")
    }
	
    db, err := plugin.Instance(nil).GetDBByTenantId(mdValue[0])
    if err != nil {
        panic("get tenant db err")
    }
    
    return db.WithContext(ctx)
}
```

