package db

import (
	"database/sql"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
)

type DBPool struct {
	db           *sql.DB
	maxConns     int
	activeConns  int
	mutex        sync.Mutex
	connTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func NewDBPool(dataSourceName string, maxConns int) (*DBPool, error) {
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns / 2)
	db.SetConnMaxLifetime(time.Hour)

	return &DBPool{
		db:           db,
		maxConns:     maxConns,
		connTimeout:  5 * time.Second,
		readTimeout:  30 * time.Second,
		writeTimeout: 30 * time.Second,
	}, nil
}

func (p *DBPool) BeginTx() (*sql.Tx, error) {
	p.mutex.Lock()
	if p.activeConns >= p.maxConns {
		p.mutex.Unlock()
		return nil, ErrTooManyConnections
	}
	p.activeConns++
	p.mutex.Unlock()

	tx, err := p.db.Begin()
	if err != nil {
		p.mutex.Lock()
		p.activeConns--
		p.mutex.Unlock()
		return nil, err
	}

	return tx, nil
}

func (p *DBPool) ReleaseTx(tx *sql.Tx, err error) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.activeConns--

	if err != nil {
		logrus.WithError(err).Error("Rolling back transaction due to error")
		return tx.Rollback()
	}

	return tx.Commit()
}

func (p *DBPool) Close() error {
	return p.db.Close()
}
