// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

package dbutil

import (
	"gitlab.com/lightmeter/controlcenter/lmsqlite3/dbconn"
	"gitlab.com/lightmeter/controlcenter/lmsqlite3/migrator"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"path"
)

func initConnPair(workspaceDirectory, filename string) (*dbconn.PooledPair, func(), error) {
	dbFilename := path.Join(workspaceDirectory, filename)

	connPair, err := dbconn.Open(dbFilename, 5)
	if err != nil {
		return nil, nil, errorutil.Wrap(err)
	}

	f := func() {
		errorutil.MustSucceed(connPair.Close(), "Closing connection on error")
	}

	return connPair, f, nil
}

func MigratorRunDown(workspaceDirectory string, databaseName string, version int64) error {
	connPair, closeHandler, err := initConnPair(workspaceDirectory, databaseName+".db")
	if err != nil {
		return errorutil.Wrap(err)
	}

	defer closeHandler()

	if err := migrator.RunDownTo(connPair.RwConn.DB, databaseName, version); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}
