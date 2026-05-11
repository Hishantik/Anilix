package provider

import (
	"github.com/anilix/anilix/provider/allanime"
	"github.com/anilix/anilix/provider/mock"
	"github.com/anilix/anilix/source"
)

type Provider struct {
	ID           string
	Name         string
	UsesHeadless bool
	IsCustom     bool
	CreateSource func() (source.Source, error)
}

func (p Provider) String() string {
	return p.Name
}

var providers = []*Provider{
	{
		ID:   mock.ID,
		Name: mock.Name,
		CreateSource: func() (source.Source, error) {
			return &mock.Mock{}, nil
		},
	},
	{
		ID:   allanime.ID,
		Name: allanime.Name,
		CreateSource: func() (source.Source, error) {
			return &allanime.Allanime{}, nil
		},
	},
}

func All() []*Provider {
	return providers
}

func Get(name string) (*Provider, bool) {
	for _, p := range providers {
		if p.Name == name {
			return p, true
		}
	}
	return nil, false
}