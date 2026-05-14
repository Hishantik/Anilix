package Allanime

// GraphQL queries for AllAnime API

const queryShows = `query($search: SearchInput, $limit: Int, $page: Int, $translationType: VaildTranslationTypeEnumType, $countryOrigin: VaildCountryOriginEnumType) {
	shows(search: $search, limit: $limit, page: $page, translationType: $translationType, countryOrigin: $countryOrigin) {
		edges {
			node {
				_id
				name
				thumbnail
				availableEpisodes
			}
			cursor
		}
	}
}`

const queryShowEpisodes = `query($showId: String!) {
	show(_id: $showId) {
		_id
		name
		thumbnail
		availableEpisodesDetail {
			sub
			dub
		}
	}
}`

const queryEpisodeSources = `query($showId: String!, $translationType: VaildTranslationTypeEnumType!, $episodeString: String!) {
	episode(showId: $showId, translationType: $translationType, episodeString: $episodeString) {
		sourceUrls
	}
}`

// Additional useful queries

const querySearchById = `query($id: String!) {
	show(_id: $id) {
		_id
		name
		thumbnail
		description
		genre
		releaseDate
		availableEpisodes
		availableEpisodesDetail {
			sub
			dub
		}
	}
}`

const queryAllProviders = `query($episodeId: String!) {
	episode(_id: $episodeId) {
		links {
			name
			url
			type
		}
	}
}`