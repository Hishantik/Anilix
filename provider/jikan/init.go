package jikan

import "github.com/anilix/anilix/provider"

func init() {
	provider.Register(&provider.Provider{
		ID:           "jikan",
		Name:         "Jikan",
		UsesHeadless: false,
		IsCustom:     false,
		CreateSource: func() (interface{}, error) {
			return NewJikanProvider(), nil
		},
	})
}