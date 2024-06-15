package service

import (
	"github.com/0xelden/common-libs-go/gateway/controller/resolver"
	"github.com/0xelden/common-libs-go/gateway/controller/resolver/manual"
	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/0xelden/common-libs-go/shared"
)

func NewResolver() (resolv resolver.Resolver, errx serror.SError) {
	switch helper.Env(shared.AppCluster, shared.ClusterLocal) {

	default:
		resolv, errx = manual.NewManualResolver()
	}

	if errx != nil {
		return resolv, errx
	}

	errx = resolv.Register()
	if errx != nil {
		return resolv, errx
	}

	return resolv, errx
}
