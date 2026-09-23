package module2workers

// config data obj
type Config struct {
	Development          bool   `json:"Development"`
	ServerPort           string `json:"ServerPort"`
	TlsCertPath          string `json:"TlsCertPath"`
	TlsKeyPath           string `json:"TlsKeyPath"`
	CookieEncryptionKey  string `json:"CookieEncryptionKey"`
	Module2ServerPort    string `json:"Module2ServerPort"`
	Module2ApiKey        string `json:"Module2ApiKey"`
	Module2GUIPath       string `json:"Module2GUIPath"`
	SmtpHost             string `json:"SmtpHost"`
	SmtpUser             string `json:"SmtpUser"`
	PrivateKeyPassphrase string `json:"PrivateKeyPassphrase"`
}

// tables entry to DB
type HotTable struct {
	ID       int    `json:"id"`
	Capacity int    `json:"capacity"`
	Floor    int    `json:"floor"`
	Name     string `json:"name"`
	Position string `json:"position"`
}

// requests of availability
type ReqData struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Floor     int    `json:"floor"`
	Capacity  int    `json:"capacity"`
	IsHot     bool   `json:"is_hot"`
}

type Reserva struct {
	Mesas     []HotTable `json:"mesas"`
	StartTime string     `json:"startTime"`
	EndTime   string     `json:"endTime"`
}

type ConsultaColisiones struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	TableID int    `json:"table_id"`
}

type ReservaEntry struct {
	TableID    int      `json:"table_id"`
	Name       string   `json:"name"`
	Concept    string   `json:"concept"`
	StartDate  string   `json:"start_date"`
	EndDate    string   `json:"end_date"`
	Email      string   `json:"email"`
	Asistentes []string `json:"asistentes"`
}

// type for auth and role parse
type AuthCookieClaims struct {
	Sub  string `json:"sha256username"`
	Role string `json:"role"`
	Exp  int64  `json:"exp"`
}

type ReservaModification struct {
	ReservationID int    `json:"reservation_id"`
	Concept       string `json:"concept"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
}

// type for log parsing
type Log struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
}

// Desk management structs
type OccDTO struct {
	ID                      int     `json:"id"`
	StartDate               string  `json:"start_date"`
	EndDate                 *string `json:"end_date"`
	Person                  string  `json:"person_name"`
	UserID                  *int    `json:"user_id"`
	GuestName               *string `json:"guest_name"`
	GuestHost               *string `json:"guest_host"`
	GuestProgram            *string `json:"guest_program"`
	GuestRole               *string `json:"guest_role"`
	ContractID              *int    `json:"contract_id"`
	ContractVinculationType *string `json:"contract_vinculation_type"`
	ContractType            *string `json:"contract_type"`
	ContractPosition        *string `json:"contract_position"`
	ContractStartDate       *string `json:"contract_start_date"`
	ContractEndDate         *string `json:"contract_end_date"`
}

type DeskDTO struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	Floor        int      `json:"floor"`
	IsHot        bool     `json:"is_hot"`
	Room         string   `json:"room_uab_code"`
	Assignments  []OccDTO `json:"assignments"`
	Reservations []OccDTO `json:"reservations"`
	Releases     []OccDTO `json:"releases"`
}

type PersonDTO struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

type ContractDTO struct {
	ID              int     `json:"id"`
	PeopleID        int     `json:"people_id"`
	VinculationType *string `json:"vinculation_type"`
	Type            *string `json:"type"`
	Position        *string `json:"position"`
	StartDate       string  `json:"start_date"`
	EndDate         *string `json:"end_date"`
}

type ReleaseDeskReq struct {
	DeskID    int    `json:"desk_id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type AssignDeskReq struct {
	DeskID       int    `json:"desk_id"`
	PersonID     int    `json:"person_id"`
	ContractID   int    `json:"contract_id"`
	Guest        bool   `json:"is_guest"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	GuestName    string `json:"guest_name"`
	GuestHost    string `json:"guest_host"`
	GuestProgram string `json:"guest_program"`
	GuestRole    string `json:"guest_role"`
}

type ReservableDeskReq struct {
	DeskID int `json:"desk_id"`
}
