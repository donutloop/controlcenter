package migrations

import (
	"database/sql"
	"gitlab.com/lightmeter/controlcenter/lmsqlite3/migrator"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
)

func init() {
	migrator.AddMigration("deliverydb", "1_delivery_tables.go", up, down)
}

func up(tx *sql.Tx) error {
	// TODO: investigate, via profiling, which fields deserve to have indexes apart from the obvious ones.
	sql := `
create table deliveries (
	status integer not null,
	delivery_ts integer not null,
	direction integer not null,
	sender_domain_part_id integer not null,
	recipient_domain_part_id integer not null,
	orig_recipient_domain_part_id integer, -- optional
	message_id integer not null,
	conn_ts_begin integer, -- FIXME: due a parser issue with NOQUEUE, we sometimes don't have this value :-(
	queue_ts_begin integer not null,
	orig_msg_size integer not null,
	processed_msg_size integer not null,
	nrcpt integer not null,
	delivery_server_id integer not null,
	delay double not null,
	delay_smtpd double not null,
	delay_cleanup double not null,
	delay_qmgr double not null,
	delay_smtp double not null,
	next_relay_id integer, -- optional
	sender_local_part text not null,
	recipient_local_part text not null,
	orig_recipient_local_part text,
	client_hostname text, -- FIXME: due a parser issue with NOQUEUE, we sometimes don't have this value :-(
	client_ip blob, -- FIXME: due a parser issue with NOQUEUE, we sometimes don't have this value :-(
	dsn text not null
);

create index deliveries_ts_index on deliveries(delivery_ts, direction);
create index deliveries_status_delivery_ts_index on deliveries(status, delivery_ts, direction);

create table messageids (
	value text not null
);

create index messageids_index on messageids(value);

create table remote_domains (
	domain text not null
);

create index remote_domains_index on remote_domains(domain);

create table next_relays (
	port integer not null,
	hostname text not null,
	ip blob not null
);

create index next_relays_index on next_relays(hostname, ip, port);

create table delivery_server (
	hostname text not null
);

create index delivery_server_hostname_index on delivery_server(hostname);

`
	if _, err := tx.Exec(sql); err != nil {
		return errorutil.Wrap(err)
	}

	return nil
}

func down(tx *sql.Tx) error {
	return nil
}
