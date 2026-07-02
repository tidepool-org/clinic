package test

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/store/postgres"
	"github.com/tidepool-org/clinic/test"
)

const (
	// templateDatabaseName is migrated once (guarded by an advisory lock so
	// parallel ginkgo processes don't race) and each suite gets a cheap copy
	// via CREATE DATABASE ... TEMPLATE.
	templateDatabaseName = "clinic_template"
	// templateLockId serializes template creation and cloning across
	// processes; cloning requires the template to have no other connections.
	templateLockId = 794613
)

var pgConfig *postgres.Config

func SetupPostgres() {
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()

	config, err := postgres.NewConfig()
	Expect(err).ToNot(HaveOccurred())

	adminConfig := *config
	adminConfig.DatabaseName = "postgres"
	admin, err := pgx.Connect(ctx, adminConfig.ConnectionString())
	Expect(err).ToNot(HaveOccurred())
	defer admin.Close(ctx)

	_, err = admin.Exec(ctx, "SELECT pg_advisory_lock($1)", templateLockId)
	Expect(err).ToNot(HaveOccurred())
	defer func() {
		_, err := admin.Exec(ctx, "SELECT pg_advisory_unlock($1)", templateLockId)
		Expect(err).ToNot(HaveOccurred())
	}()

	var templateExists bool
	err = admin.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", templateDatabaseName).Scan(&templateExists)
	Expect(err).ToNot(HaveOccurred())
	if !templateExists {
		_, err = admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", templateDatabaseName))
		Expect(err).ToNot(HaveOccurred())
	}

	templateConfig := *config
	templateConfig.DatabaseName = templateDatabaseName
	Expect(postgres.Migrate(ctx, &templateConfig)).To(Succeed())

	databaseName := strings.ToLower(fmt.Sprintf("clinic_test_%s_%d", test.Faker.RandomStringWithLength(10), ginkgo.GinkgoParallelProcess()))
	_, err = admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", databaseName, templateDatabaseName))
	Expect(err).ToNot(HaveOccurred())

	Expect(os.Setenv("TIDEPOOL_POSTGRES_DATABASE_NAME", databaseName)).To(Succeed())
	config.DatabaseName = databaseName
	pgConfig = config
}

func TeardownPostgres() {
	ctx, cancel := context.WithTimeout(context.Background(), mongoTimeout)
	defer cancel()

	Expect(pgConfig).ToNot(BeNil())

	adminConfig := *pgConfig
	adminConfig.DatabaseName = "postgres"
	admin, err := pgx.Connect(ctx, adminConfig.ConnectionString())
	Expect(err).ToNot(HaveOccurred())
	defer admin.Close(ctx)

	// FORCE terminates lingering connections so teardown ordering relative
	// to the fx app shutdown doesn't matter.
	_, err = admin.Exec(ctx, fmt.Sprintf("DROP DATABASE %s WITH (FORCE)", pgConfig.DatabaseName))
	Expect(err).ToNot(HaveOccurred())
	pgConfig = nil
}

// GetTestPostgresConfig returns the configuration of the suite's postgres
// database for direct assertions on mirrored rows.
func GetTestPostgresConfig() *postgres.Config {
	Expect(pgConfig).ToNot(BeNil())
	return pgConfig
}
