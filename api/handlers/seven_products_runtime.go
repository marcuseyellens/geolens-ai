package handlers

import (
	"sync"

	"github.com/answerlens/answerlens/internal/db"
	"github.com/answerlens/answerlens/internal/sevenproducts"
)

// 第六批依赖采用包级注入（与既有 SetDeltaRuntime 模式一致），
// 避免在 Platform/Server 结构体里改动第五批定稿字段。
var sevenRuntime struct {
	mu   sync.RWMutex
	db   *db.DB
	flag *sevenproducts.Flag
}

// SetSevenProductsRuntime 由 main.go 注入第六批 DB 与 flag。
func SetSevenProductsRuntime(d *db.DB, f *sevenproducts.Flag) {
	sevenRuntime.mu.Lock()
	sevenRuntime.db = d
	sevenRuntime.flag = f
	sevenRuntime.mu.Unlock()
}

func sevenFlag() *sevenproducts.Flag {
	sevenRuntime.mu.RLock()
	defer sevenRuntime.mu.RUnlock()
	return sevenRuntime.flag
}

func sevenDB() *db.DB {
	sevenRuntime.mu.RLock()
	defer sevenRuntime.mu.RUnlock()
	return sevenRuntime.db
}
