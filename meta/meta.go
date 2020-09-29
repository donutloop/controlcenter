package meta

import (
	"database/sql"
	"encoding/json"
	"errors"
	"gitlab.com/lightmeter/controlcenter/lmsqlite3/dbconn"
	"gitlab.com/lightmeter/controlcenter/lmsqlite3/migrator"
	_ "gitlab.com/lightmeter/controlcenter/meta/migrations"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"reflect"
)

var (
	ErrNoSuchKey = errors.New("No Such Key")
)

type MetadataHandler struct {
	conn dbconn.ConnPair
}

func NewMetaDataHandler(conn dbconn.ConnPair, databaseName string) (*MetadataHandler, error) {
	if err := migrator.Run(conn.RwConn.DB, databaseName); err != nil {
		return nil, errorutil.Wrap(err)
	}

	return &MetadataHandler{conn}, nil
}

func (h *MetadataHandler) Close() error {
	return h.conn.Close()
}

type Item struct {
	Key   string
	Value interface{}
}

type Result struct {
}

func (h *MetadataHandler) Store(items []Item) (Result, error) {
	tx, err := h.conn.RwConn.Begin()

	if err != nil {
		return Result{}, errorutil.Wrap(err)
	}

	defer func() {
		if err != nil {
			errorutil.MustSucceed(tx.Rollback(), "")
		}
	}()

	r, err := Store(tx, items)

	if err != nil {
		return Result{}, err
	}

	err = tx.Commit()

	if err != nil {
		return Result{}, errorutil.Wrap(err)
	}

	return r, nil
}

func Store(tx *sql.Tx, items []Item) (Result, error) {
	for _, i := range items {
		var id int
		err := tx.QueryRow(`select rowid from meta where key = ?`, i.Key).Scan(&id)

		query, args := func() (string, []interface{}) {
			if errors.Is(err, sql.ErrNoRows) {
				return `insert into meta(key, value) values(?, ?)`, []interface{}{i.Key, i.Value}
			}

			return `update meta set value = ? where rowid = ?`, []interface{}{i.Value, id}
		}()

		if _, err := tx.Exec(query, args...); err != nil {
			return Result{}, errorutil.Wrap(err)
		}
	}

	return Result{}, nil
}

func retrieve(h *MetadataHandler, key string, value interface{}) error {
	err := h.conn.RoConn.QueryRow(`select value from meta where key = ?`, key).Scan(value)

	if err == nil {
		return nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoSuchKey
	}

	return errorutil.Wrap(err)
}

func (h *MetadataHandler) Retrieve(key string) (interface{}, error) {
	var v interface{}

	if err := retrieve(h, key, &v); err != nil {
		return nil, errorutil.Wrap(err)
	}

	return v, nil
}

func (h *MetadataHandler) StoreJson(key interface{}, value interface{}) (Result, error) {
	stmt, err := h.conn.RwConn.Prepare(`insert into meta(key, value) values(?, ?)`)

	if err != nil {
		return Result{}, errorutil.Wrap(err)
	}

	defer func() { errorutil.MustSucceed(stmt.Close(), "") }()

	jsonBlob, err := json.Marshal(value)
	if err != nil {
		return Result{}, errorutil.Wrap(err)
	}

	_, err = stmt.Exec(key, string(jsonBlob))

	if err != nil {
		return Result{}, errorutil.Wrap(err)
	}

	return Result{}, nil
}

func (h *MetadataHandler) RetrieveJson(key string, values interface{}) error {
	reflectValues := reflect.ValueOf(values)

	if reflectValues.Kind() != reflect.Ptr {
		panic("values isn't a pointer")
	}

	var v string
	if err := retrieve(h, key, &v); err != nil {
		return errorutil.Wrap(err)
	}

	if err := json.Unmarshal([]byte(v), values); err != nil {
		return errorutil.Wrap(err, "could not Unmarshal values")
	}

	return nil
}
