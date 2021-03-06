// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

package migrations

import (
	"database/sql"
	"gitlab.com/lightmeter/controlcenter/lmsqlite3/migrator"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
)

func init() {
	migrator.AddMigration("logtracker", "1_tracking.go", upCreateTrackingTables, downCreateTrackingTables)
}

func upCreateTrackingTables(tx *sql.Tx) error {
	// TODO: investigate, via profiling, which fields deserve to have indexes apart from the obvious ones.
	sql := `
create table queues (
	id integer primary key,
	connection_id integer not null,
	usage_counter integer not null, -- incremented whenever anything links the queue, and decrement otherwise
	messageid_id integer,
	queue text not null
);

create index queue_text_index on queues(queue);

create table results (
	id integer primary key,
	queue_id integer not null
);

create table result_data (
	id integer primary key,
	result_id integer not null,
	key integer not null,
	value blob not null
);

create index result_data_result_id_index on result_data(result_id);

create table messageids (
	id integer primary key,
	usage_counter integer not null,
	value text not null
);

create index messageids_text on messageids(value);

create table queue_parenting (
	id integer primary key,
	orig_queue_id integer not null,
	new_queue_id integer not null,
	parenting_type integer not null
);

-- TODO: check if the two indexes are really needed!
create index queue_parenting_new_queue_id_index on queue_parenting(new_queue_id);
create index queue_parenting_orig_queue_id_index on queue_parenting(orig_queue_id);

create table queue_data (
	id integer primary key,
	queue_id integer not null,
	key integer not null,
	value blob not null
);

create index queue_data_queue_id_index on queue_data(queue_id);

create table connections (
	id integer primary key,
	pid_id integer not null,
	usage_counter integer not null
);

create index connections_pid_id_index on connections(pid_id);

create table connection_data (
	id integer primary key,
	connection_id integer not null,
	key integer not null,
	value blob not null
);

create index connection_data_connection_id_index on connection_data(connection_id);

create table pids (
	id integer primary key,
	pid integer not null,
	usage_counter integer not null,
	host text not null
);

create index pids_id_index on pids(host, pid);

-- TODO: move this table to a different file, or better, to memory!
create table notification_queues (
	id integer primary key,
	result_id integer not null,
	line integer not null,
	filename text
);

create table processed_queues (
	id integer primary key,
	queue_id integer not null
);

create index processed_queues_index on processed_queues(queue_id);

`
	if _, err := tx.Exec(sql); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

func downCreateTrackingTables(tx *sql.Tx) error {
	return nil
}
