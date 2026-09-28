package goworkers

import "fmt"

// insert line and hash on db
func (l *Log) persist(module string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	l.PreviousHash = getPreviousLogHash(tx)
	l.Hash = hashString(l.createLogString())

	_, err = tx.Exec(
		`INSERT INTO logs (id_module, timestamp, log_content, code, hash, previous_hash)
         VALUES (?, ?, ?, ?, ?, ?)`,
		module,
		l.Timestamp,
		l.Msg,
		l.Code,
		l.Hash,
		l.PreviousHash,
	)
	if err != nil {
		return err
	}

	return nil
}

// generate log string based on log fields
func (l *Log) createLogString() string {
	return fmt.Sprintf(
		"PreviousHash:%s;Timestamp:%d;Msg:%s;Code:%d",
		l.PreviousHash,
		l.Timestamp,
		l.Msg,
		l.Code,
	)
}

// error call from controller
func AddControllerLog(msg string, code int) {
	l := Log{
		Timestamp: getUnixTimestamp(),
		Code:      code,
		Msg:       encodeB64String([]byte(encryptLogWithGPG(msg))),
	}
	_ = l.persist("controller")
}

// error call from module api
func AddModuleLog(module, msg string, code int) error {
	l := Log{
		Timestamp: getUnixTimestamp(),
		Msg:       msg, // we recieve the log already cypher from each module
		Code:      code,
	}
	return l.persist(module)
}
