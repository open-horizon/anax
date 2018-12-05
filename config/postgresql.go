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
	SSLMode  string
}

func (p PostgresqlConfig) MakeConnectionString() (string, string) {

	// By default we will use SSL to communicate to the DB.
	sslMode := "require"
	if p.SSLMode != "" {
		sslMode = p.SSLMode
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", p.Host, p.Port, p.User, p.Password, p.DBName, sslMode)
	traceString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", p.Host, p.Port, p.User, "********", p.DBName, sslMode)

	if p.Password == "" {
		connStr = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s", p.Host, p.Port, p.User, p.DBName, sslMode)
		traceString = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s", p.Host, p.Port, p.User, p.DBName, sslMode)
	}

	return connStr, traceString
}
