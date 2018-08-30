package config

import (
	"fmt"
)

type PostgresqlConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	NoSSL    bool
}

func (p PostgresqlConfig) MakeConnectionString() (string, string) {

	// By default we will use SSL to communicate to the DB.
	sslMode := "enable"
	if p.NoSSL {
		sslMode = "disable"
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", p.Host, p.Port, p.User, p.Password, p.DBName, sslMode)
	traceString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", p.Host, p.Port, p.User, "********", p.DBName, sslMode)

	if p.Password == "" {
		connStr = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s", p.Host, p.Port, p.User, p.DBName, sslMode)
		traceString = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s", p.Host, p.Port, p.User, p.DBName, sslMode)
	}

	return connStr, traceString
}
