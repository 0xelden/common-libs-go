package repo

import "github.com/0xelden/common-libs-go/gateway/models"

type CompositeRepository interface {
	GetRedisByID(models.CompositeID) (models.Composite, error)
}
