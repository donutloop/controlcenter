package dbconn

import (
	"database/sql"
	"fmt"
	_ "gitlab.com/lightmeter/controlcenter/lmsqlite3"
	"gitlab.com/lightmeter/controlcenter/util"
)

type ConnPair struct {
	RoConn *sql.DB
	RwConn *sql.DB
}

func (c *ConnPair) Close() error {
	readerError := c.RoConn.Close()
	writerError := c.RwConn.Close()

	if writerError == nil {
		if readerError != nil {
			return util.WrapError(readerError)
		}

		// no errors at all
		return nil
	}

	// here we know that writeError != nil

	if readerError == nil {
		return util.WrapError(writerError)
	}

	// Both errors exist. We lose the erorrs, keeping only the message, which is ok for now
	return fmt.Errorf("RW: %v, RO: %v", writerError, readerError)
}

func NewConnPair(filename string) (ConnPair, error) {
	writer, err := sql.Open("lm_sqlite3", `file:`+filename+`?mode=rwc&cache=private&_loc=auto&_journal=WAL`)

	if err != nil {
		return ConnPair{}, util.WrapError(err)
	}

	defer func() {
		if err != nil {
			util.MustSucceed(writer.Close(), "Closing RW connection on error")
		}
	}()

	reader, err := sql.Open("lm_sqlite3", `file:`+filename+`?mode=ro&cache=shared&_query_only=true&_loc=auto&_journal=WAL`)

	if err != nil {
		return ConnPair{}, util.WrapError(err)
	}

	return ConnPair{RoConn: reader, RwConn: writer}, nil
}
