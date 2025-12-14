package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Seann-Moser/credentials/oauth/oserver"
	"github.com/Seann-Moser/credentials/session"
	"github.com/Seann-Moser/credentials/user"
	"github.com/Seann-Moser/hypr-config-manager/pkg/hchandler"
	"github.com/Seann-Moser/hypr-config-manager/pkg/hyprconfig"
	"github.com/Seann-Moser/hypr-config-manager/pkg/utils"
	"github.com/Seann-Moser/mserve"
	"github.com/Seann-Moser/rbac"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Config struct {
	MongoURL      string
	MongoDatabase string
	Secret        string
	Origin        string
	OriginName    string
	RPId          string
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "A brief description of your command",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		mongoCreds, err := utils.LoadConfig[options.Credential](cmd, "mongo")
		if err != nil {
			return err
		}
		cfg, err := utils.LoadConfig[Config](cmd, "c")
		if err != nil {
			return err
		}
		sslConfig, err := utils.LoadConfig[mserve.SSLConfig](cmd, "c")
		if err != nil {
			return err
		}
		mongoDB, err := mongo.Connect(cmd.Context(), options.Client().ApplyURI(cfg.MongoURL).SetAuth(mongoCreds))
		if err != nil {
			return err
		}

		rbacManager, err := rbac.NewMongoStoreManager(cmd.Context(), mongoDB.Database(cfg.MongoDatabase))
		if err != nil {
			return err
		}

		oServer := oserver.NewMongoServer(mongoDB.Database(cfg.MongoDatabase))

		ses := session.NewClient(oServer, rbacManager, []byte(cfg.Secret), 24*time.Hour)
		s := mserve.NewServer("HyprlandConfigManager", rbacManager, []string{}, ses, sslConfig)
		userServer, err := user.NewServer(
			user.NewMongoDBStore(
				mongoDB,
				cfg.MongoDatabase,
				"user",
			), rbacManager, []byte(cfg.Secret),
			cfg.RPId,
			cfg.OriginName,
			cfg.Origin,
		)

		configManager, err := hyprconfig.NewConfigManager(
			mongoDB.Database(cfg.MongoDatabase).Collection("configs"),
			mongoDB.Database(cfg.MongoDatabase).Collection("favorites"),
			mongoDB.Database(cfg.MongoDatabase).Collection("state"),
			mongoDB.Database(cfg.MongoDatabase).Collection("allowed_programs"),
		)
		if err != nil {
			return err
		}

		hcHandler, _ := hchandler.NewHandler(configManager)
		err = s.AddEndpoints(ctx, hcHandler.GetEndpoints()...)
		if err != nil {
			return err
		}

		err = s.SetupOServer(ctx, oServer).
			SetupRbac(ctx).
			SetupSlog(slog.LevelWarn).
			//SetupMetrics().
			SetupUserLogin(ctx, userServer).
			HealthCheck("/healthz", nil).
			GenerateOpenAPIDocs().
			Run(ctx)
		if err != nil {
			return err
		}
		return nil
	}}

func init() {
	err := setServerFlags(serveCmd)
	if err != nil {
		fmt.Println(err)
	}
	rootCmd.AddCommand(serveCmd)
}

func setServerFlags(cmd *cobra.Command) error {
	mongoCfg, err := utils.BindFlags(&options.Credential{
		Password: "default",
		Username: "admin",
	}, "mongo")
	if err != nil {
		return err
	}

	cmd.Flags().AddFlagSet(mongoCfg)
	cfg, err := utils.BindFlags(&Config{
		MongoURL:      "mongodb://mongodb:27017",
		MongoDatabase: "local",
		Secret:        "default",
		Origin:        "http://localhost:3000",
		OriginName:    "HyprConfigManager",
		RPId:          "localhost.com",
	}, "c")
	if err != nil {
		return err
	}

	cmd.Flags().AddFlagSet(cfg)

	cfg, err = utils.BindFlags(&mserve.SSLConfig{
		Port: 8080,
	}, "c")
	if err != nil {
		return err
	}

	cmd.Flags().AddFlagSet(cfg)
	return err
}
