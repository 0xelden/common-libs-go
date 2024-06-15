package resolver

import "github.com/0xelden/common-libs-go/serror"

type OptionKey string

type GenerateOptions map[OptionKey]interface{}

type Resolver interface {
	Register() (errx serror.SError)
	GenerateURL(name string, port string) (url string)
}
