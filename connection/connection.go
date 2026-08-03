package connection

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/0xelden/common-libs-go/helper"
	"github.com/0xelden/common-libs-go/objstorage"
	"github.com/0xelden/common-libs-go/serror"
	"github.com/0xelden/common-libs-go/shared"
	"github.com/ClickHouse/clickhouse-go"
	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"github.com/hibiken/asynq"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	_ "modernc.org/sqlite"
)

func ConnectClickhouse() (result *sql.DB, serr serror.SError) {
	dsn := fmt.Sprintf(
		// v2 = clickhouse://username:password@host1:9000,host2:9000/database?dial_timeout=200ms&max_execution_time=60
		// v1 = "tcp://127.0.0.1:9000?username=&compress=true&debug=true"
		"tcp://%s:%s?username=%s&password=%s&database=%s&compress=true&debug=true",
		helper.Env("CH_HOST", "192.168.30.160"),
		helper.Env("CH_PORT", "9000"),
		helper.Env("CH_USER", "default"),
		helper.Env("CH_PASS", ""),
		helper.Env("CH_DB", "uptrace"),
	)
	connect, err := sql.Open("clickhouse", dsn)
	//connect, err := otelsql.Open("clickhouse", dsn, otelsql.WithAttributes(semconv.DBSystemVertica))
	if err != nil {
		return result, serror.NewFromError(err)
	}
	if err := connect.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			fmt.Println("clickhouse exception", exception)
		}
		return result, serror.NewFromError(err)
	}
	connect.SetMaxIdleConns(5)
	connect.SetMaxOpenConns(10)
	//connect.SetConnMaxLifetime(time.Hour)
	return connect, nil
}

func ConnectPgsql(dbName, user, pass, host, port, ssl string) (*sql.DB, serror.SError) {
	dsn := fmt.Sprintf(
		// postgres://username:password@localhost:5432/database_name
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&connect_timeout=3",
		user,
		pass,
		host,
		port,
		dbName,
		ssl,
	)

	db, err := otelsql.Open("postgres", dsn, otelsql.WithAttributes(semconv.DBSystemPostgreSQL))
	if err != nil {
		return nil, serror.NewFromError(err)
	}

	err = db.Ping()
	if err != nil {
		return nil, serror.NewFromError(err)
	}

	maxOpen := int(helper.StringToInt(helper.Env(shared.DBConnMaxOpen, "128"), 128))
	maxIdle := int(helper.StringToInt(helper.Env(shared.DBConnMaxIdle, "16"), 16))
	lifetime := int(helper.StringToInt(helper.Env(shared.DBConnLifetime, "5"), 5))

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(time.Duration(lifetime) * time.Minute)

	return db, nil
}

func ConnectMysql(dbName, user, pass, host, port, ssl string) (*sql.DB, serror.SError) {
	dsn := fmt.Sprintf(
		// username:password@protocol(address)/dbname?param=value
		"%s:%s@tcp(%s:%s)/%s?tls=%s&timeout=3s",
		user,
		pass,
		host,
		port,
		dbName,
		ssl,
	)

	db, err := otelsql.Open("mysql", dsn, otelsql.WithAttributes(semconv.DBSystemMySQL))
	if err != nil {
		return nil, serror.NewFromError(err)
	}

	err = db.Ping()
	if err != nil {
		return nil, serror.NewFromError(err)
	}

	maxOpen := int(helper.StringToInt(helper.Env(shared.DBConnMaxOpen, "128"), 128))
	maxIdle := int(helper.StringToInt(helper.Env(shared.DBConnMaxIdle, "16"), 16))
	lifetime := int(helper.StringToInt(helper.Env(shared.DBConnLifetime, "5"), 5))

	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(time.Duration(lifetime) * time.Minute)

	return db, nil
}

func ConnectPG() (*sql.DB, serror.SError) {
	return ConnectPgsql(
		helper.Env(shared.DBName, "default"),
		helper.Env(shared.DBUser, "root"),
		helper.Env(shared.DBPwd, ""),
		helper.Env(shared.DBHost, ""),
		helper.Env(shared.DBPort, "26357"),
		helper.Env(shared.DBSSLMode, "disable"),
	)
}

func ConnectAllPgsql() (result map[string]*sql.DB, serr serror.SError) {
	result = map[string]*sql.DB{}
	result[shared.Default], serr = ConnectPG()
	if serr != nil {
		return nil, serr
	}
	for i := 0; i < 10; i++ {
		code := helper.Env(fmt.Sprintf("DB.%d.CODE", i), "")
		if code == "" {
			continue
		}
		result[code], serr = ConnectPgsql(
			helper.Env(fmt.Sprintf("DB.%d.NAME", i), helper.Env("DB_NAME")),
			helper.Env(fmt.Sprintf("DB.%d.USER", i), helper.Env("DB_USER")),
			helper.Env(fmt.Sprintf("DB.%d.PWD", i), helper.Env("DB_PWD")),
			helper.Env(fmt.Sprintf("DB.%d.HOST", i), helper.Env("DB_HOST")),
			helper.Env(fmt.Sprintf("DB.%d.PORT", i), helper.Env("DB_PORT")),
			helper.Env(fmt.Sprintf("DB.%d.SSL_MODE", i), helper.Env("DB_SSL_MODE", "disable")),
		)
		if serr != nil {
			return nil, serr
		}
	}
	return result, nil
}

// ConnectAllPgsqlReplica opens a read-replica connection for the default
// portal and every DB.<n>.CODE portal found in primary (the map returned by
// ConnectAllPgsql), returning a map keyed for db.NewSqlDriverMulti:
//   - shared.Default / "<code>"            -> passthrough to primary[...]
//   - shared.Default+":replica" / "<code>:replica" -> the replica connection
//
// The default portal's replica falls back to DB_REPLICA_HOST/DB_REPLICA_PORT
// -> DB_HOST/DB_PORT when unset. Each portal's replica falls back to
// DB.<n>.REPLICA_HOST/DB.<n>.REPLICA_PORT -> that portal's own primary
// host/port, so per-portal replicas are opt-in.
//
// Pair the ":replica" keys with api.SetReplicaContext, which selects them via
// shared.ExecuteIn while staying aware of the request's shared.X_PortalCode.
func ConnectAllPgsqlReplica(primary map[string]*sql.DB) (result map[string]*sql.DB, serr serror.SError) {
	result = map[string]*sql.DB{}

	defaultReplica, serr := ConnectPgsql(
		helper.Env("DB_NAME"),
		helper.Env("DB_USER"),
		helper.Env("DB_PWD"),
		helper.Env("DB_REPLICA_HOST", helper.Env("DB_HOST")),
		helper.Env("DB_REPLICA_PORT", helper.Env("DB_PORT")),
		helper.Env("DB_SSL_MODE", "disable"),
	)
	if serr != nil {
		return nil, serr
	}
	result[shared.Default] = primary[shared.Default]
	result[shared.Default+":replica"] = defaultReplica

	for i := 0; i < 10; i++ {
		code := helper.Env(fmt.Sprintf("DB.%d.CODE", i), "")
		if code == "" {
			continue
		}

		host := helper.Env(fmt.Sprintf("DB.%d.HOST", i), helper.Env("DB_HOST"))
		port := helper.Env(fmt.Sprintf("DB.%d.PORT", i), helper.Env("DB_PORT"))

		var replica *sql.DB
		replica, serr = ConnectPgsql(
			helper.Env(fmt.Sprintf("DB.%d.NAME", i), helper.Env("DB_NAME")),
			helper.Env(fmt.Sprintf("DB.%d.USER", i), helper.Env("DB_USER")),
			helper.Env(fmt.Sprintf("DB.%d.PWD", i), helper.Env("DB_PWD")),
			helper.Env(fmt.Sprintf("DB.%d.REPLICA_HOST", i), host),
			helper.Env(fmt.Sprintf("DB.%d.REPLICA_PORT", i), port),
			helper.Env(fmt.Sprintf("DB.%d.SSL_MODE", i), helper.Env("DB_SSL_MODE", "disable")),
		)
		if serr != nil {
			return nil, serr
		}

		result[code] = primary[code]
		result[code+":replica"] = replica
	}

	return result, nil
}

func ConnectSqliteInmemory() (*sql.DB, serror.SError) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, serror.NewFromError(err)
	}
	return db, nil
}

func ConnectRedis() (*redis.Client, serror.SError) {
	client := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),
		Password: "",
		DB:       0,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, serror.NewFromError(err)
	}
	return client, nil
}

// ConnectAsynq builds the *asynq.RedisClientOpt used by both worker.NewConfig
// and asynq.NewClient, from JOB_REDIS_HOST/PORT/PWD/DB (DB defaults to 1, not
// 0, so the job queue doesn't share a keyspace with ConnectRedis's session/
// cache DB by default). Connectivity is verified via asynq.Inspector.Servers,
// which also logs any already-running worker servers found on that Redis.
func ConnectAsynq() (*asynq.RedisClientOpt, serror.SError) {
	var (
		db   = helper.StringToInt(helper.Env("JOB_REDIS_DB"), 1)
		host = helper.Env("JOB_REDIS_HOST", "127.0.0.1")
		port = helper.Env("JOB_REDIS_PORT", "6379")
		pass = helper.Env("JOB_REDIS_PWD", "")
		addr = fmt.Sprintf("%s:%s", host, port)
		opts = &asynq.RedisClientOpt{Addr: addr, Password: pass, DB: int(db)}
		insp = asynq.NewInspector(opts)
	)

	res, err := insp.Servers()
	if err != nil {
		return nil, serror.NewFromError(err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer insp.Close()

	for _, r := range res {
		w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
		_, _ = fmt.Fprintln(w, "Job server\t: "+r.ID)
		_, _ = fmt.Fprintln(w, "Job host\t: "+r.Host)
		_, _ = fmt.Fprintln(w, "Active workers\t: "+helper.ToString(r.ActiveWorkers))
		_, _ = fmt.Fprintln(w, "Status\t: "+r.Status)
		_ = w.Flush()
	}

	return opts, nil
}

// ConnectObjectStorage connects to the object-storage backend selected by the
// STORAGE_DRIVER env var: "minio" (default) or "azure". All connection vars
// share the STORAGE_ prefix — STORAGE_MINIO_HOST/ID/SECRET/SSL and
// STORAGE_AZURE_CONNECTION_STRING (or STORAGE_AZURE_ACCOUNT/KEY/ENDPOINT).
// The Azurite emulator is supported via
// STORAGE_AZURE_CONNECTION_STRING=UseDevelopmentStorage=true. Legacy unprefixed
// names (MINIO_*, AZURE_STORAGE_*) are read as fallback for services that
// have not migrated their .env yet.
func ConnectObjectStorage() (objstorage.Client, serror.SError) {
	driver := strings.ToLower(strings.TrimSpace(helper.Env("STORAGE_DRIVER", objstorage.DriverMinio)))
	switch driver {
	case "", objstorage.DriverMinio:
		client, serr := ConnectMinio()
		if serr != nil {
			return nil, serr
		}
		return objstorage.NewMinio(client), nil
	case objstorage.DriverAzure, "azblob":
		connString := helper.Env("STORAGE_AZURE_CONNECTION_STRING", helper.Env("AZURE_STORAGE_CONNECTION_STRING"))
		if strings.TrimSpace(connString) != "" {
			return objstorage.NewAzureFromConnectionString(connString)
		}
		account := helper.Env("STORAGE_AZURE_ACCOUNT", helper.Env("AZURE_STORAGE_ACCOUNT"))
		key := helper.Env("STORAGE_AZURE_KEY", helper.Env("AZURE_STORAGE_KEY"))
		if strings.TrimSpace(account) == "" || strings.TrimSpace(key) == "" {
			return nil, serror.New("STORAGE_DRIVER=azure requires STORAGE_AZURE_CONNECTION_STRING or STORAGE_AZURE_ACCOUNT + STORAGE_AZURE_KEY")
		}
		return objstorage.NewAzureSharedKey(account, key, helper.Env("STORAGE_AZURE_ENDPOINT", helper.Env("AZURE_STORAGE_ENDPOINT")))
	default:
		return nil, serror.Newf("unsupported STORAGE_DRIVER %q (expected %q or %q)", driver, objstorage.DriverMinio, objstorage.DriverAzure)
	}
}

// ConnectMinio connects directly to MinIO from the STORAGE_MINIO_* env vars,
// falling back to the legacy unprefixed MINIO_* names.
//
// Deprecated: prefer ConnectObjectStorage, which also supports Azure Blob
// Storage via STORAGE_DRIVER.
func ConnectMinio() (*minio.Client, serror.SError) {
	endpoint := helper.Env("STORAGE_MINIO_HOST", helper.Env("MINIO_HOST"))
	accessKeyID := helper.Env("STORAGE_MINIO_ID", helper.Env("MINIO_ID"))
	secretAccessKey := helper.Env("STORAGE_MINIO_SECRET", helper.Env("MINIO_SECRET"))
	useSSL := helper.ToBool(helper.Env("STORAGE_MINIO_SSL", helper.Env("MINIO_SSL")), true)

	// Initialize minio client object.
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, serror.NewFromError(err)
	}

	if _, err := client.HealthCheck(time.Second * 5); err != nil {
		return nil, serror.NewFromError(err)
	}
	if client.IsOffline() {
		return nil, serror.New("Object storage is offline")
	}

	return client, nil
}
