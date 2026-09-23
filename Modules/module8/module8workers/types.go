package module8workers

// config data obj
type Config struct {
	Development            bool   `json:"Development"`
	ServerPort             string `json:"ServerPort"`
	TlsCertPath            string `json:"TlsCertPath"`
	TlsKeyPath             string `json:"TlsKeyPath"`
	CookieEncryptionKey    string `json:"CookieEncryptionKey"`
	Module8ServerPort      string `json:"Module8ServerPort"`
	Module8ApiKey          string `json:"Module8ApiKey"`
	Module8GUIPath         string `json:"Module8GUIPath"`
	SmtpHost               string `json:"SmtpHost"`
	SmtpUser               string `json:"SmtpUser"`
	PrivateKeyPassphrase   string `json:"PrivateKeyPassphrase"`
	PrivateKeyPath         string `json:"LogCypherPrivKeyFilePath"`
	LogCypherKeyPassphrase string `json:"LogPrivateKeyPassphrase"`
	PDFSignerCommand       string `json:"PDFSignerCommand"`
	PDFCertificatePath     string `json:"PDFCertificatePath"`
	PDFCertificatePassword string `json:"PDFCertificatePassword"`
	PDFSignatureReason     string `json:"PDFSignatureReason"`
	PDFSignatureLocation   string `json:"PDFSignatureLocation"`
}

// type for auth and role parse
type AuthCookieClaims struct {
	Sub  string `json:"sha256username"`
	Role string `json:"role"`
	Exp  int64  `json:"exp"`
}

// type for log parsing
type Log struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
}

// file security
var allowedExtensions = map[string]bool{
	".pdf":  true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".webp": true,
}

var allowedMimeTypes = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
	"image/webp":      true,
}

type SavedFile struct {
	OriginalName string `json:"originalName"`
	StoredName   string `json:"storedName"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	MIME         string `json:"mime"`
}

// budgeting request
type RequestPayload struct {
	Categories               []string
	TravelFrom               string
	TravelTo                 string
	Institution              string
	TravelSince              string
	TravelUntil              string
	TravelPurpose            string
	TravelProject            string
	TravelIP                 int
	TravelObservations       string
	TravelContactPhone       string
	TravelContactPhoneOK     string
	TravelPassportType       string
	TravelPassportNumber     string
	TravelPassportExpiration string
	TravelPassportOK         string
	TravelLuggageType        string
	TravelLuggageKg          *int
	TravelSeatPreference     string
	TravelTimePreference     string
	TravelArea               string
	RegistrationEvent        string
	RegistrationStartDay     string
	RegistrationEndDay       string
	RegistrationFile         string
	RegistrationPayment      string
	RegistrationProject      string
	RegistrationIP           int
	RegistrationObservations string
	AccomodationWhere        string
	AccomodationSince        string
	AccomodationUntil        string
	AccomodationPurpose      string
	AccomodationProject      string
	AccomodationIP           int
	AccomodationObservations string
	OtherDescription         string
	OtherPriceRange          string
	OtherProject             string
	OtherIP                  int
	EquipmentCategory        string
	EquipmentOtherDescr      string
	EquipmentPriceRange      string
	EquipmentDescription     string
	EquipmentProject         string
	EquipmentIP              int
}

type ExistingPart struct {
	ID                  int
	CategoryID          int
	ProjectID           string
	IPID                int
	Purpose             string
	Observations        string
	FromDay             string
	UntilDay            string
	TravelFromPlace     string
	WherePlace          string
	Institution         string
	RegistrationInvoice string
	RegistrationType    string
	EquipmentCategory   string
	OtherPrice          string
	LuggageType         string
	LuggageWeight       *int
	SeatPreference      string
	TimePreference      string
	TravelArea          string
}

var CATEGORY_ID_TO_KEY = map[int]string{
	1: "travel",
	2: "registration",
	3: "accommodation",
	4: "equipment",
	5: "other",
}

var CATEGORY_ID_TO_KEY_CATALAN = map[int]string{
	1: "viatge",
	2: "inscripció",
	3: "allotjament",
	4: "equipament",
	5: "altres despeses",
}

type TravelPassportInfo struct {
	DocType    string `json:"doc_type"`
	DocNumber  string `json:"doc_number"`
	Expiration string `json:"expiration"`
}

type TravelContactInfoResponse struct {
	OK               bool                `json:"ok"`
	Error            string              `json:"error,omitempty"`
	HasPhone         bool                `json:"hasPhone"`
	Phone            string              `json:"phone,omitempty"`
	PhoneSource      string              `json:"phoneSource,omitempty"`
	HasValidDocument bool                `json:"hasValidDocument"`
	HasValidPassport bool                `json:"hasValidPassport"`
	HasPassportData  bool                `json:"hasPassportData"`
	Passport         *TravelPassportInfo `json:"passport,omitempty"`
}

type BudgetPartMailItem struct {
	CategoryName        string
	ProjectName         string
	IPName              string
	Purpose             string
	Observations        string
	FromDay             string
	UntilDay            string
	FromPlace           string
	WherePlace          string
	TravelArea          string
	LuggageType         string
	LuggageWeight       string
	SeatPreference      string
	TimePreference      string
	RegistrationType    string
	RegistrationInvoice string
	EquipmentCategory   string
	OtherPrice          string
}

type BudgetAcceptedByProjectsMailData struct {
	Name              string
	Surname           string
	ResearcherName    string
	ResearcherSurname string
	PIName            string
	PISurname         string
	ProjectName       string
	UpdatedAt         string
	RequestSummary    string

	ResearcherDocumentType       string
	ResearcherDocumentNumber     string
	ResearcherDocumentExpiration string
	ResearcherBirthDate          string
	ResearcherPhone              string
	ResearcherCorporateEmail     string

	BudgetParts []BudgetPartMailItem
}

// Mail to IP at creation
type IPConfirmationMailData struct {
	PIName            string
	PISurname         string
	ResearcherName    string
	ResearcherSurname string
	ProjectName       string
	CreatedAt         string
	RequestSummary    string
}

// Mail to researcher at creation
type WorkerConfirmationMailData struct {
	Name                  string
	Surname               string
	RequestProject        string
	PrincipalInvestigator string
	CreatedAt             string
	RequestSummary        string
}

type WorkerRejectionMailData struct {
	Name           string
	Surname        string
	RejName        string
	RejSurname     string
	ProjectName    string
	CreatedAt      string
	UpdatedAt      string
	RequestSummary string
	Message        string
}

type WorkerLastMailData struct {
	Name           string
	Surname        string
	ProjectName    string
	CreatedAt      string
	UpdatedAt      string
	RequestSummary string
}

// Mail to HR at travel resolution
type RRHHTravelMailData struct {
	ResearcherName    string
	ResearcherSurname string
	PIName            string
	PISurname         string
	ProjectName       string
	UpdatedAt         string
	TravelPurpose     string
	Destination       string
	Institution       string
	DepartureDate     string
	ReturnDate        string
	HasInvitation     bool
}

// Mail to projects when IP accepts

type ProjectsConfirmationMailData struct {
	ProjectsName                  string
	ProjectsSurname               string
	ResearcherName                string
	ResearcherSurname             string
	PIName                        string
	PISurname                     string
	ProjectName                   string
	UpdatedAt                     string
	RequestSummary                string
	RegistrationEvent             string
	RegistrationObservations      string
	RegistrationHasSupportingFile bool
	RegistrationRequiresMeeting   bool
}

type ManagementConfirmationMailData struct {
	ManagementName    string
	ManagementSurname string
	ResearcherName    string
	ResearcherSurname string
	PIName            string
	PISurname         string
	ProjectName       string
	UpdatedAt         string
	RequestSummary    string
	CategoryName      string
	PriceRange        string
	EquipmentCategory string
	Purpose           string
	Observations      string
}

type IPRejectedMailData struct {
	PIName            string
	PISurname         string
	ResearcherName    string
	ResearcherSurname string
	ProjectName       string
	UpdatedAt         string
	RequestSummary    string
	Message           string
}

type ProjectsRejectedMailData struct {
	ProjectsName      string
	ProjectsSurname   string
	ResearcherName    string
	ResearcherSurname string
	PIName            string
	PISurname         string
	ProjectName       string
	UpdatedAt         string
	RequestSummary    string
	Message           string
}

type rejectionRecipientInfo struct {
	ID      int
	Name    string
	Surname string
	Email   string
}

type AccountingInformsMailData struct {
	Name              string
	Surname           string
	ResearcherName    string
	ResearcherSurname string
	PIName            string
	PISurname         string
	ProjectName       string
	UpdatedAt         string
	EstimatedAmount   string
	RequestSummary    string
}

type CanceledMailData struct {
	Name              string
	Surname           string
	ResearcherName    string
	ResearcherSurname string
	PIName            string
	PISurname         string
	ProjectName       string
	RequestType       string
	CreatedAt         string
	UpdatedAt         string
	RequestSummary    string
	Message           string
}
type CancelFlowInfo struct {
	CombinedID        int
	RequestType       string
	ProjectName       string
	PIName            string
	PISurname         string
	CreatedAt         string
	UpdatedAt         string
	RequestSummary    string
	ResearcherID      int
	ResearcherName    string
	ResearcherSurname string
	ResearcherEmail   string

	HasIPResponse        bool
	HasProjectsResponse  bool
	HasAccOrITResponse   bool
	HasEquipmentCategory bool
}

type CancelRecipient struct {
	To      string
	Name    string
	Surname string
	RoleKey string
}

type InvoiceProvidedMailData struct {
	Name              string
	Surname           string
	ResearcherName    string
	ResearcherSurname string
	PIName            string
	PISurname         string
	ProjectName       string
	UpdatedAt         string
	RequestSummary    string
}
type InvoiceFlowInfo struct {
	CombinedID          int
	ProjectName         string
	PIName              string
	PISurname           string
	ResearcherName      string
	ResearcherSurname   string
	RequestSummary      string
	HasIPResponse       bool
	HasProjectsResponse bool
	HasAccOrITResponse  bool
}

// Attendance certificate tracking for travel / registration requests.
type AttendanceCertificatePendingItem struct {
	CombinedID     int    `json:"combinedId"`
	InternalID     string `json:"internalId"`
	ProjectName    string `json:"projectName"`
	RequestSummary string `json:"requestSummary"`
	EndDate        string `json:"endDate"`
	LastReminderAt string `json:"lastReminderAt"`
	ReminderCount  int    `json:"reminderCount"`
}

type AttendanceCertificateCandidate struct {
	CombinedID  int      `json:"combinedId"`
	InternalID  string   `json:"internalId"`
	PeopleID    int      `json:"peopleId"`
	ProjectName string   `json:"projectName"`
	UntilDay    string   `json:"untilDay"`
	Categories  []string `json:"categories"`
	Purposes    []string `json:"purposes"`
}
type AttendanceCertificateReminderMailData struct {
	Name           string
	Surname        string
	InternalID     string
	ProjectName    string
	RequestSummary string
	EndDate        string
	Today          string
}
