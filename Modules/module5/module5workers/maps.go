package module5workers

import (
	"fmt"
	"strings"
)

type CodeMapper struct {
	MssqlToUneix map[string]string
	UneixToMssql map[string]string
}

func NewCodeMapper(mssqlToUneix map[string]string) CodeMapper {
	rev := make(map[string]string, len(mssqlToUneix))
	for ms, ux := range mssqlToUneix {
		rev[ux] = ms
	}
	return CodeMapper{
		MssqlToUneix: mssqlToUneix,
		UneixToMssql: rev,
	}
}

func MapToUneix(mapper CodeMapper, v interface{}) string {
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	if s == "" {
		return ""
	}
	if u, ok := mapper.ToUneix(s); ok {
		return u
	}
	return ""
}

func (m CodeMapper) ToUneix(ms string) (string, bool) {
	ux, ok := m.MssqlToUneix[ms]
	return ux, ok
}

func (m CodeMapper) ToMssql(ux string) (string, bool) {
	ms, ok := m.UneixToMssql[ux]
	return ms, ok
}

type UNEIXMappers struct {
	Status CodeMapper
	Gender CodeMapper
}

var Mappers = UNEIXMappers{
	Status: NewCodeMapper(map[string]string{
		"Undergraduate Student":          "5",
		"Graduate (Degree)":              "6",
		"Postgraduate (Master's Degree)": "7",
		"PhD Student":                    "7",
		"Postdoc":                        "1",
		"Senior Researcher":              "1",
		"Doctor":                         "1",
	}),
	Gender: NewCodeMapper(map[string]string{
		"Male":              "H",
		"Female":            "D",
		"Non-binary":        "",
		"Prefer not to say": "",
	}),
}
