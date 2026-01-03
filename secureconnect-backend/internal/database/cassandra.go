package database

import (
	"github.com/gocql/gocql"
)

type CassandraDB struct {
	Session *gocql.Session
}

func NewCassandraDB(hosts []string, keyspace string) (*CassandraDB, error) {
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.Quorum
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, err
	}
	return &CassandraDB{Session: session}, nil
}

func (c *CassandraDB) Close() {
	c.Session.Close()
}
