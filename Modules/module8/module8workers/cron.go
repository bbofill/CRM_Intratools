// cron.go
package module8workers

import (
	"fmt"
	"net/http"
	"time"

	cron "github.com/robfig/cron/v3"
)

var attendanceCertificatesCron *cron.Cron

func InitAttendanceCertificatesCron(apiKey string, client *http.Client) {
	loc, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		createLog(fmt.Sprintf("Failed to load location for attendance certificates cron: %v", err), 1, apiKey, client, nil)
		loc = time.UTC
	}

	attendanceCertificatesCron = cron.New(cron.WithLocation(loc))

	// Cada día a las 08:15.
	// Revisa todos los viajes/inscripciones finalizados y envía recordatorios si toca.
	_, err = attendanceCertificatesCron.AddFunc("15 8 * * *", func() {
		createLog("ATTENDANCE CERTIFICATES CRON ACTIVATED", 0, apiKey, client, nil)

		pending, err := collectPendingAttendanceCertificates(0, true, apiKey, client, nil)
		if err != nil {
			createLog(fmt.Sprintf("Error in attendance certificates cron: %v", err), 1, apiKey, client, nil)
			return
		}

		createLog(
			fmt.Sprintf("Attendance certificates cron executed. Pending certificates: %d", len(pending)),
			0,
			apiKey,
			client,
			nil,
		)
	})

	if err != nil {
		createLog(fmt.Sprintf("Failed to schedule attendance certificates cron: %v", err), 1, apiKey, client, nil)
		return
	}

	attendanceCertificatesCron.Start()
}
