package Allanime

import "github.com/hishantik/anilix/provider"

func init() {
	provider.Register(&provider.Provider{
		ID:           "allanime",
		Name:         "AllAnime",
		UsesHeadless: false,
		IsCustom:     false,
		CreateSource: func() (interface{}, error) {
			return NewAllanimeProvider(), nil
		},
	})
}