package manual

import (
	"fmt"

	"github.com/0xelden/common-libs-go/gateway/controller/resolver"
	"github.com/0xelden/common-libs-go/serror"
)

type manualResolver struct{}

func NewManualResolver() (res resolver.Resolver, errx serror.SError) {
	res = manualResolver{}
	return res, errx
}

func (ox manualResolver) GenerateURL(service string, port string) (url string) {
	url = fmt.Sprintf("%s:%s", service, port)
	return url
}

func (ox manualResolver) Register() (errx serror.SError) {
	return errx
}
