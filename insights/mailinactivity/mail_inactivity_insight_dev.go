// +build dev !release

package mailinactivity

import (
	"database/sql"
	"gitlab.com/lightmeter/controlcenter/data"
	"gitlab.com/lightmeter/controlcenter/insights/core"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
)

// Executed only on development builds, for better developer experience
func (d *detector) GenerateSampleInsight(tx *sql.Tx, c core.Clock) error {
	if err := generateInsight(tx, c, d.creator, data.TimeInterval{From: c.Now().Add(-d.options.LookupRange), To: c.Now()}); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}
