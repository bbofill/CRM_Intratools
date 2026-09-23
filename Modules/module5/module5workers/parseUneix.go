package module5workers

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func sanitizeORCID(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}

	s = strings.ToLower(s)

	// If it's a URL, keep only what's after "orcid.org/"
	if idx := strings.Index(s, "orcid.org/"); idx != -1 {
		s = s[idx+len("orcid.org/"):]
	}

	// Remove query params / fragments if present
	if q := strings.IndexAny(s, "?#"); q != -1 {
		s = s[:q]
	}

	// Remove everything except digits and X (X is allowed as last check character)
	// Also remove hyphens/spaces implicitly.
	reKeep := regexp.MustCompile(`[^0-9x]`)
	s = reKeep.ReplaceAllString(s, "")

	// Must be exactly 16 chars (15 digits + [0-9|X])
	if len(s) != 16 {
		return ""
	}

	// Basic structure check: first 15 must be digits
	for i := 0; i < 15; i++ {
		if s[i] < '0' || s[i] > '9' {
			return ""
		}
	}
	// last char can be digit or 'x'
	last := s[15]
	if !((last >= '0' && last <= '9') || last == 'x') {
		return ""
	}

	// Return uppercase X if needed
	return strings.ToUpper(s)
}

// helper min para el printf
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func pickVisitorIdentity(r map[string]interface{}) (firstName, surname1, surname2 string) {
	// 1) Nombre: wu.first_name si existe, si no, no_webuser (primer token)
	if v, ok := r["first_name"].(string); ok && strings.TrimSpace(v) != "" {
		firstName = strings.TrimSpace(v)
	} else if v, ok := r["no_webuser"].(string); ok && strings.TrimSpace(v) != "" {
		fn, _, _ := splitNameAndSurnames(v)
		firstName = fn
	}

	// 2) Apellidos
	if v, ok := r["surname"].(string); ok && strings.TrimSpace(v) != "" {
		surname1, surname2 = split1or2(v)
	} else if v, ok := r["no_webuser_last_name"].(string); ok && strings.TrimSpace(v) != "" {
		surname1, surname2 = split1or2(v)
	} else if v, ok := r["no_webuser"].(string); ok && strings.TrimSpace(v) != "" {
		_, s1, s2 := splitNameAndSurnames(v)
		surname1, surname2 = s1, s2
	}

	return firstName, surname1, surname2
}

// splits fields where name and surname are together
func split1or2(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	parts := strings.Fields(s)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

func splitNameAndSurnames(noWebuser string) (firstName, s1, s2 string) {
	noWebuser = strings.TrimSpace(noWebuser)
	if noWebuser == "" {
		return "", "", ""
	}
	parts := strings.Fields(noWebuser)
	if len(parts) == 1 {
		return parts[0], "", ""
	}
	if len(parts) == 2 {
		return parts[0], parts[1], ""
	}
	// len >= 3
	return parts[0], parts[1], strings.Join(parts[2:], " ")
}

func validateAcademicGrade(u map[string]interface{}, academicGrade interface{}, peopleID int, hasValidDoctorate bool) []string {
	var missing []string

	// Normalizar academicGrade
	var gradeVal string
	switch g := academicGrade.(type) {
	case string:
		gradeVal = strings.TrimSpace(g)
	case float64:
		gradeVal = fmt.Sprintf("%.0f", g)
	default:
		gradeVal = ""
	}

	if gradeVal == "" {
		return missing
	}

	gradeNum, _ := strconv.Atoi(gradeVal)

	// === Nivel 1: Bachelor / Master ===
	if gradeNum == 1 || gradeNum == 2 || gradeNum == 6 || gradeNum == 7 {
		if isEmpty(u["grade_master_doctorate"]) {
			missing = append(missing, "grade_master_doctorate")
		}
		if isEmpty(u["graduation_university"]) {
			missing = append(missing, "graduation_university")
		}
		if isEmpty(u["graduation_country"]) {
			missing = append(missing, "graduation_country")
		}
	}

	// === Nivel 2: Doctorate (solo si academic_grade == 1) ===
	if gradeNum == 1 {
		val, ok := u["grade_master_doctorate"].(string)
		if ok && strings.TrimSpace(val) == "Doctorate" {
			if isEmpty(u["graduation_university"]) {
				missing = append(missing, "graduation_university")
			}
			if isEmpty(u["graduation_country"]) {
				missing = append(missing, "graduation_country")
			}
		} else if !hasValidDoctorate {
			// Solo marcamos como “missing doctorate record” si aún no hay uno válido
			missing = append(missing, "Missing doctorate record. Please, add it in education from Module 4.")
		}
	}

	return missing
}
