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
type TenantConn struct{
	Conf *conf.Data
}

func (c TenantConn) CreateDBConn(tenant string) (db *gorm.DB, err error) {
    db, err = gorm.Open(mysql.Open(fmt.Sprintf(c.Conf.Database.Dsn, tenant)), &gorm.Config{})
    if err != nil {
        return
    }
    sqlDB, err := db.DB()
    if err != nil {
        sqlDB.Close()
        return
    }
    sqlDB.SetMaxIdleConns(int(c.Conf.Database.MaxIdle))
    sqlDB.SetMaxOpenConns(int(c.Conf.Database.MaxOpen))
    sqlDB.SetConnMaxLifetime(c.Conf.Database.ConnLifetime.AsDuration())
    
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

//conf := &config.Data{...}
mt := plugin.Instance()
mt.Register("tenant_tag", TenantConn{Conf: conf})
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
	
    db, err := plugin.Instance().GetDBByTenantId(mdValue[0])
    if err != nil {
        panic("get tenant db err")
    }
    
    return db.WithContext(ctx)
}
```

