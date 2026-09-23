package module1workers

// config data obj
type Config struct {
	Development          bool   `json:"Development"`
	ServerPort           string `json:"ServerPort"`
	TlsCertPath          string `json:"TlsCertPath"`
	TlsKeyPath           string `json:"TlsKeyPath"`
	CookieEncryptionKey  string `json:"CookieEncryptionKey"`
	Module1ServerPort    string `json:"Module1ServerPort"`
	Module1ApiKey        string `json:"Module1ApiKey"`
	Module1GUIPath       string `json:"Module1GUIPath"`
	SmtpHost             string `json:"SmtpHost"`
	SmtpUser             string `json:"SmtpUser"`
	PrivateKeyPassphrase string `json:"PrivateKeyPassphrase"`
	OutlookTenantID      string `json:"OutlookTenantID"`
	OutlookClientID      string `json:"OutlookClientID"`
	OutlookClientSecret  string `json:"OutlookClientSecret"`
	OutlookAdminEmail    string `json:"OutlookAdminEmail"`
	EWSURL               string `json:"EWSURL"`
}

// type for auth and role parse
type AuthCookieClaims struct {
	Sub  string `json:"sha256username"`
	Role string `json:"role"`
	Exp  int64  `json:"exp"`
}

type Room struct {
	ID       int      `json:"id"`
	Name     string   `json:"name"`
	UABCode  string   `json:"uab_code"`
	Category []string `json:"category"`
}

type RoomsResponse struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Rooms     []Room `json:"rooms"`
}

type ReqDataModify struct {
	ReservationId int    `json:"excludeReservationId"`
	Concept       string `json:"concept"`
	Room          string `json:"roomId"`
	StartTime     string `json:"start_date"`
	EndTime       string `json:"end_date"`
}

type ReqData struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Category  string `json:"category"`

	Frequency string `json:"frequency"`
	Weekdays  []int  `json:"weekdays"`
	RepeatBy  string `json:"repeat_by"` // month_day, month_weekday, year_day, year_weekday
	EndRepeat string `json:"end_repetition"`
}

type ReqRepeated struct {
	RoomID      int          `json:"room_id"`
	Frequency   string       `json:"frequency"`
	Occurrences []Occurrence `json:"occurrences"`
	Concept     string       `json:"concept"`
	Email       string       `json:"email"`
}

type ReservaEntry struct {
	ID             int    `json:"id"`
	RoomID         int    `json:"room_id"`
	Name           string `json:"name"`
	StartDate      string `json:"start_date"`
	EndDate        string `json:"end_date"`
	Email          string `json:"email"`
	Concept        string `json:"concept"`
	OutlookEventID string `json:"outlook_event_id"`
}

type Occurrence struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type CalendarEvent struct {
	Subject string
	Start   string
	End     string
}

type Collision struct {
	Index  int    `json:"index"`
	Start  string `json:"start"`
	End    string `json:"end"`
	Reason string `json:"reason"`
}
