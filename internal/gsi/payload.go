package gsi

// GSIPayload 代表 Valve CS2 GSI 推送的完整数据根结构
type GSIPayload struct {
	Provider   *ProviderSection `json:"provider,omitempty"`
	Map        *MapSection      `json:"map,omitempty"`
	Round      *RoundSection    `json:"round,omitempty"`
	Player     *PlayerSection   `json:"player,omitempty"`
	Previously *PreviouslyState `json:"previously,omitempty"`
	Added      *PreviouslyState `json:"added,omitempty"`
	Auth       map[string]any   `json:"auth,omitempty"`
}

type ProviderSection struct {
	Name      string `json:"name"`
	AppID     int    `json:"appid"`
	Version   int    `json:"version"`
	SteamID   string `json:"steamid"`
	Timestamp int64  `json:"timestamp"`
}

type MapSection struct {
	Mode        string      `json:"mode"`
	Name        string      `json:"name"`
	Phase       string      `json:"phase"` // "warmup", "intermission", "gameover", "live"
	Round       int         `json:"round"`
	TeamCT      TeamSummary `json:"team_ct"`
	TeamT       TeamSummary `json:"team_t"`
	NumMatches  int         `json:"num_matches_to_win_series"`
	CurrentSpec int         `json:"current_spectators"`
}

type TeamSummary struct {
	Score                   int    `json:"score"`
	ConsecutiveRoundLosses  int    `json:"consecutive_round_losses"`
	TimeoutsRemaining       int    `json:"timeouts_remaining"`
	MatchesWonThisSeries    int    `json:"matches_won_this_series"`
}

type RoundSection struct {
	Phase   string `json:"phase"` // "freezetime", "live", "over"
	WinTeam string `json:"win_team,omitempty"`
	Bomb    string `json:"bomb,omitempty"` // "planted", "exploded", "defused"
}

type PlayerSection struct {
	SteamID      string       `json:"steamid"`
	Name         string       `json:"name"`
	ObserverSlot int          `json:"observer_slot"`
	Team         string       `json:"team"` // "CT", "T"
	Activity     string       `json:"activity"` // "menu", "playing"
	State        *PlayerState `json:"state"`
}

type PlayerState struct {
	Health      int  `json:"health"`
	Armor       int  `json:"armor"`
	Helmet      bool `json:"helmet"`
	DefuseKit   bool `json:"defusekit"`
	Flashed     int  `json:"flashed"`
	Smoked      int  `json:"smoked"`
	Burning     int  `json:"burning"`
	Money       int  `json:"money"`
	RoundKills  int  `json:"round_kills"`
	RoundKillHS int  `json:"round_killhs"`
	EquipValue  int  `json:"equip_value"`
}

type PreviouslyState struct {
	Player *PlayerSection `json:"player,omitempty"`
	Map    *MapSection    `json:"map,omitempty"`
	Round  *RoundSection  `json:"round,omitempty"`
}
